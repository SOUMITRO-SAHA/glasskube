package repo

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	packagesv1alpha1 "github.com/glasskube/glasskube/api/v1alpha1"
	"github.com/glasskube/glasskube/internal/httperror"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var defaultRepositoryURL = "https://packages.dl.glasskube.dev/packages/"

var idxMutex sync.Mutex
var idxUpdate time.Time
var packageRepoIndex PackageRepoIndex

func UpdatePackageManifest(pi *packagesv1alpha1.PackageInfo) (err error) {
	var manifest packagesv1alpha1.PackageManifest
	if err = FetchPackageManifest(pi.Spec.RepositoryUrl, pi.Spec.Name, pi.Spec.Version, &manifest); err != nil {
		return
	}
	pi.Status.Manifest = &manifest
	pi.Status.Version = pi.Spec.Version
	return nil
}

func FetchLatestPackageManifest(repoURL, name string, target *packagesv1alpha1.PackageManifest) (version string, err error) {
	var versions PackageIndex
	if err = FetchPackageIndex(repoURL, name, &versions); err != nil {
		return
	} else {
		version = versions.LatestVersion
	}
	err = FetchPackageManifest(repoURL, name, version, target)
	return
}

func FetchPackageManifest(repoURL, name, version string, target *packagesv1alpha1.PackageManifest) error {
	if url, err := GetPackageManifestURL(repoURL, name, version); err != nil {
		return err
	} else {
		return fetchYAMLOrJSON(url, target)
	}
}

func FetchPackageIndex(repoURL, name string, target *PackageIndex) error {
	if url, err := getPackageIndexURL(repoURL, name); err != nil {
		return err
	} else {
		return fetchYAMLOrJSON(url, target)
	}
}

func FetchPackageRepoIndex(repoURL string, target *PackageRepoIndex) error {
	if url, err := getPackageRepoIndexURL(repoURL); err != nil {
		return err
	} else {
		return fetchYAMLOrJSON(url, target)
	}
}

func GetLatestVersion(repoURL string, pkgName string) (string, error) {
	idxMutex.Lock()
	defer idxMutex.Unlock()
	if len(packageRepoIndex.Packages) == 0 || idxUpdate.Add(5*time.Minute).Before(time.Now()) {
		if err := FetchPackageRepoIndex(repoURL, &packageRepoIndex); err != nil {
			return "", err
		}
		idxUpdate = time.Now()
	}
	for _, pkg := range packageRepoIndex.Packages {
		if pkg.Name == pkgName {
			return pkg.LatestVersion, nil
		}
	}
	return "", nil
}

func fetchYAMLOrJSON(url string, target any) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if err = httperror.CheckResponse(resp); err != nil {
		return fmt.Errorf("failed to fetch %v: %w", url, err)
	}
	return yaml.NewYAMLOrJSONDecoder(resp.Body, 4096).Decode(target)
}

func getPackageRepoIndexURL(repoURL string) (string, error) {
	return url.JoinPath(getBaseURL(repoURL), "index.yaml")
}

func getPackageIndexURL(repoURL, name string) (string, error) {
	return url.JoinPath(getBaseURL(repoURL), url.PathEscape(name), "versions.yaml")
}

func GetPackageManifestURL(repoURL, name, version string) (string, error) {
	pathSegments := []string{url.PathEscape(name), url.PathEscape(version), "package.yaml"}
	return url.JoinPath(getBaseURL(repoURL), pathSegments...)
}

func getBaseURL(explicitRepositoryURL string) string {
	if len(explicitRepositoryURL) > 0 {
		return explicitRepositoryURL
	} else {
		return defaultRepositoryURL
	}
}
