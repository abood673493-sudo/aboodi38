// Package yaml is a local placeholder for gopkg.in/yaml.v3, used only via a
// go.mod replace directive so that `go mod tidy` can compute the module
// graph in a network environment where gopkg.in is unreachable. It is a
// transitive TEST-ONLY dependency of a dependency (stretchr/testify, pulled
// in by pgx's own test suite) that this project never imports, builds, or
// runs - this stub is never linked into ssh-manager-panel.
package yaml

func Marshal(in interface{}) ([]byte, error) {
	return nil, nil
}

func Unmarshal(in []byte, out interface{}) error {
	return nil
}
