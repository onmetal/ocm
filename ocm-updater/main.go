package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	v2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/codec"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	flag "github.com/spf13/pflag"
	"go.uber.org/zap"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strings"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	flag.Parse()

	zapLogger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer func() { _ = zapLogger.Sync() }()
	log := zapr.NewLogger(zapLogger)

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	for _, path := range flag.Args() {
		if err := updatePath(ctx, log, path); err != nil {
			log.Error(err, "error updating path", "Path", path)
			os.Exit(1)
		}
	}
}

func updatePath(ctx context.Context, log logr.Logger, path string) error {
	if !strings.HasSuffix(path, "/...") {
		log := log.WithValues("Directory", path)
		return updateDir(ctx, log, path)
	}

	dir := strings.TrimSuffix(path, "/...")
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		filename := filepath.Join(path, "component-descriptor.yaml")
		stat, err := os.Stat(filename)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("error stat-ing %s: %w", filename, err)
			}
			return nil
		}
		if !stat.Mode().IsRegular() {
			return nil
		}
		log := log.WithValues("Directory", path)
		return updateDir(ctx, log, path)
	})
}

func updateDir(ctx context.Context, log logr.Logger, dirname string) error {
	log.Info("Updating directory")
	filename := filepath.Join(dirname, "component-descriptor.yaml")
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read component descriptor file: %w", err)
	}

	desc := &v2.ComponentDescriptor{}
	if err := codec.Decode(data, desc); err != nil {
		return fmt.Errorf("could not decode component descriptor: %w", err)
	}

	desc, err = updateComponentDescriptor(ctx, log, DefaultGitResolver, desc)
	if err != nil {
		return fmt.Errorf("failed to update the component descriptor: %w", err)
	}

	data, err = Encode(desc)
	if err != nil {
		return fmt.Errorf("failed to encode the component descriptor: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write component descriptor to disk: %w", err)
	}
	return nil
}

func updateComponentDescriptor(ctx context.Context, log logr.Logger, gitResolver GitResolver, desc *v2.ComponentDescriptor) (*v2.ComponentDescriptor, error) {
	desc = desc.DeepCopy()
	for i := range desc.Sources {
		s := &desc.Sources[i]
		log := log.WithValues("Component", desc.Name, "Version", s.Version)
		switch s.Type {
		case v2.GitType:
			access := &v2.GitHubAccess{}
			if err := v2.FromUnstructuredObject(v2.DefaultJSONTypedObjectCodec, s.Access, access); err != nil {
				return nil, fmt.Errorf("failed to convert github access: %w", err)
			}

			latest, err := gitResolver.ResolveLatest(ctx, access.RepoURL)
			if err != nil {
				return nil, fmt.Errorf("failed to get latest tag for %s: %w", access.RepoURL, err)
			}

			log := log.WithValues("LatestVersion", latest)

			if s.Version != latest {
				log.Info("Found new version")
			}
			s.Version = fmt.Sprintf("v%s", latest)
			access.Ref = fmt.Sprintf("refs/tags/v%s", latest)
			newAccess, err := v2.NewUnstructured(access)
			if err != nil {
				return nil, fmt.Errorf("failed to create new unstructured access object: %w", err)
			}

			*s.Access = newAccess

			if access.RepoURL == desc.Name {
				latestVersion := fmt.Sprintf("v%s", latest)
				if desc.Version != latestVersion {
					log.Info("Updating component descriptor version")
				}
				desc.Version = latestVersion
			}
		default:
			return nil, fmt.Errorf("access type not supported")
		}
	}
	return desc, nil
}

type GitResolver interface {
	ResolveLatest(ctx context.Context, repoURL string) (string, error)
}

type simpleGitResolver struct {
}

func (simpleGitResolver) ResolveLatest(ctx context.Context, repoURL string) (string, error) {
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: repoURL,
		URLs: []string{addHTTPSPrefix(repoURL)},
	})

	refs, err := remote.ListContext(ctx, &git.ListOptions{})
	if err != nil {
		return "", err
	}

	var latest *semver.Version
	for _, r := range refs {
		if !r.Name().IsTag() {
			continue
		}
		v, err := semver.NewVersion(r.Name().Short())
		if err != nil {
			continue
		}
		if latest == nil || v.GreaterThan(latest) {
			latest = v
		}
	}

	if latest == nil {
		return "", nil
	}

	return latest.String(), err
}

type cachingGitResolver struct {
	gitResolver     GitResolver
	latestByRepoURL map[string]string
}

func NewCachingGitResolver(resolver GitResolver) GitResolver {
	return &cachingGitResolver{
		gitResolver:     resolver,
		latestByRepoURL: make(map[string]string),
	}
}

func (r *cachingGitResolver) ResolveLatest(ctx context.Context, repoURL string) (string, error) {
	latest, ok := r.latestByRepoURL[repoURL]
	if ok {
		return latest, nil
	}
	latest, err := r.gitResolver.ResolveLatest(ctx, repoURL)
	if err != nil {
		return "", err
	}
	r.latestByRepoURL[repoURL] = latest
	return latest, nil
}

var DefaultGitResolver = NewCachingGitResolver(simpleGitResolver{})

func Encode(obj interface{}) ([]byte, error) {
	switch v := obj.(type) {
	case *v2.ComponentDescriptor:
		v.Metadata.Version = v2.SchemaVersion
		if err := v2.DefaultComponent(v); err != nil {
			return nil, err
		}
		return yaml.Marshal(v)
	case *v2.ComponentDescriptorList:
		v.Metadata.Version = v2.SchemaVersion
		if err := v2.DefaultList(v); err != nil {
			return nil, err
		}
		return yaml.Marshal(v)
	default:
		return nil, fmt.Errorf("unrecognized object %T", obj)
	}
}

func addHTTPSPrefix(url string) string {
	if !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	return url
}
