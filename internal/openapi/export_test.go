package openapi

// CacheMetadata is a test export of cacheMetadata.
type CacheMetadata = cacheMetadata

// CacheMetadataSuffix is a test export of the cacheMetadataSuffix constant.
const CacheMetadataSuffix = cacheMetadataSuffix

// HashURL is a test export of hashURL.
func HashURL(url string) string { return hashURL(url) }

// ReadCacheMetadata is a test export of readCacheMetadata.
func ReadCacheMetadata(path string) (CacheMetadata, error) { return readCacheMetadata(path) }
