package main

import (
    "testing"
    "github.com/hashicorp/golang-lru/v2"
)

func BenchmarkNormaliseDomain(b *testing.B) {
    lruCache, _ = lru.New[string, string](lruCacheSize)
    for i := 0; i < b.N; i++ {
        normaliseDomain("subdomain.testingcom.au")
    }
}

func TestNormaliseDomain(t *testing.T) {
    lruCache, _ = lru.New[string, string](lruCacheSize)
    t.Parallel()
    if normaliseDomain("subdomain.testingcom.au") != "testingcom.au" {
        t.Error("normaliseDomain(subdomain.testingcom.au) did not return testingcom.au")
    }
    if normaliseDomain("subdomain.testingcom.co.uk") != "testingcom.co.uk" {
        t.Error("normaliseDomain(subdomain.testingcom.co.uk) did not return testingcom.co.uk")
    }
}
