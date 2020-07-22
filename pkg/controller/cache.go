package controller

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/joshvanl/version-checker/pkg/api"
	"github.com/joshvanl/version-checker/pkg/version"
)

// imageCacheItem is a single node item for the cache of a lastest image search.
type imageCacheItem struct {
	timestamp   time.Time
	latestImage *api.ImageTag
}

// getLatestImage will get the latestImage image given an image URL and
// options. If not found in the cache, or is too old, then will do a fresh
// lookup and commit to the cache.
func (c *Controller) getLatestImage(ctx context.Context, log *logrus.Entry,
	imageURL string, opts *api.Options) (*api.ImageTag, error) {

	log = c.log.WithField("cache", "getter")

	hashIndex, err := version.CalculateHashIndex(imageURL, opts)
	if err != nil {
		return nil, err
	}

	c.cacheMu.RLock()
	cacheItem, ok := c.imageCache[hashIndex]
	c.cacheMu.RUnlock()

	// Test if exists in the cache or is too old
	if !ok || cacheItem.timestamp.Add(c.cacheTimeout).Before(time.Now()) {
		c.cacheMu.Lock()
		defer c.cacheMu.Unlock()

		latestImage, err := c.versionGetter.LatestTagFromImage(ctx, opts, imageURL)
		if err != nil {
			return nil, err
		}

		// Commit to the cache
		log.Debugf("committing search: %q", hashIndex)
		c.imageCache[hashIndex] = imageCacheItem{time.Now(), latestImage}

		return latestImage, nil
	}

	log.Debugf("found search: %q", hashIndex)

	return cacheItem.latestImage, nil
}

func (c *Controller) garbageCollect(refreshRate time.Duration) {
	log := c.log.WithField("cache", "garbage_collector")
	log.Infof("starting search cache garbage collector")

	ticker := time.NewTicker(refreshRate)
	for {
		<-ticker.C

		c.cacheMu.Lock()
		now := time.Now()
		for hashIndex, cacheItem := range c.imageCache {

			// Check is cache item is fresh
			if cacheItem.timestamp.Add(c.cacheTimeout).Before(now) {

				log.Debugf("removing stale search from cache: %q",
					hashIndex)

				delete(c.imageCache, hashIndex)
			}
		}
		c.cacheMu.Unlock()
	}

	return
}