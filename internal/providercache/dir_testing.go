// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2023 HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package providercache

import (
	"log"
	"path/filepath"
	"sort"

	"github.com/opentofu/opentofu/internal/addrs"
	"github.com/opentofu/opentofu/internal/getproviders"
)

// These functions on *Dir were moved in from dir.go and should be considered deprecated and for testing use only.

// AllAvailablePackages returns a description of all of the packages already
// present in the directory. The cache entries are grouped by the provider
// they relate to and then sorted by version precedence, with highest
// precedence first.
//
// This function will return an empty result both when the directory is empty
// and when scanning the directory produces an error.
//
// The caller is forbidden from modifying the returned data structure in any
// way, even though the Go type system permits it.
//
// Only used in tests!
func (d *Dir) AllAvailablePackages() map[addrs.Provider][]CachedProvider {
	if err := d.fillMetaCache(); err != nil {
		log.Printf("[WARN] Failed to scan provider cache directory %s: %s", d.baseDir, err)
		return nil
	}

	return d.testCache
}

// ProviderLatestVersion returns the cache entry for the latest
// version of the requested provider already available in the cache, or nil if
// there are no versions of that provider available.
//
// Only used in tests!
func (d *Dir) ProviderLatestVersion(provider addrs.Provider) *CachedProvider {
	if err := d.AllAvailablePackages(); err != nil {
		return nil
	}

	entries := d.testCache[provider]
	if len(entries) == 0 {
		return nil
	}

	return &entries[0]
}

func (d *Dir) fillMetaCache() error {
	// For d.metaCache we consider nil to be different than a non-nil empty
	// map, so we can distinguish between having scanned and got an empty
	// result vs. not having scanned successfully at all yet.
	if d.testCache != nil {
		log.Printf("[TRACE] providercache.fillMetaCache: using cached result from previous scan of %s", d.baseDir)
		return nil
	}
	log.Printf("[TRACE] providercache.fillMetaCache: scanning directory %s", d.baseDir)

	allData, err := getproviders.SearchLocalDirectory(d.baseDir)
	if err != nil {
		log.Printf("[TRACE] providercache.fillMetaCache: error while scanning directory %s: %s", d.baseDir, err)
		return err
	}

	// The getproviders package just returns everything it found, but we're
	// interested only in a subset of the results:
	// - those that are for the current platform
	// - those that are in the "unpacked" form, ready to execute
	// ...so we'll filter in these ways while we're constructing our final
	// map to save as the cache.
	//
	// We intentionally always make a non-nil map, even if it might ultimately
	// be empty, because we use that to recognize that the cache is populated.
	data := make(map[addrs.Provider][]CachedProvider)

	for providerAddr, metas := range allData {
		for _, meta := range metas {
			if meta.TargetPlatform != d.targetPlatform {
				log.Printf("[TRACE] providercache.fillMetaCache: ignoring %s because it is for %s, not %s", meta.Location, meta.TargetPlatform, d.targetPlatform)
				continue
			}
			if _, ok := meta.Location.(getproviders.PackageLocalDir); !ok {
				// PackageLocalDir indicates an unpacked provider package ready
				// to execute.
				log.Printf("[TRACE] providercache.fillMetaCache: ignoring %s because it is not an unpacked directory", meta.Location)
				continue
			}

			packageDir := filepath.Clean(string(meta.Location.(getproviders.PackageLocalDir)))

			log.Printf("[TRACE] providercache.fillMetaCache: including %s as a candidate package for %s %s", meta.Location, providerAddr, meta.Version)
			data[providerAddr] = append(data[providerAddr], CachedProvider{
				Provider:   providerAddr,
				Version:    meta.Version,
				PackageDir: filepath.ToSlash(packageDir),
			})
		}
	}

	// After we've built our lists per provider, we'll also sort them by
	// version precedence so that the newest available version is always at
	// index zero. If there are two versions that differ only in build metadata
	// then it's undefined but deterministic which one we will select here,
	// because we're preserving the order returned by SearchLocalDirectory
	// in that case..
	for _, entries := range data {
		sort.SliceStable(entries, func(i, j int) bool {
			// We're using GreaterThan rather than LessThan here because we
			// want these in _decreasing_ order of precedence.
			return entries[i].Version.GreaterThan(entries[j].Version)
		})
	}

	d.testCache = data
	return nil
}
