//go:build !windows

package docsearch

func activateDocSearchWindowImpl() bool {
	return false
}
