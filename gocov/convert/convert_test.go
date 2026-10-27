package convert

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/logangeorge01/gocov"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertProfiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.com/coveragefixture\n\ngo 1.27.0\n",
		"fixture.go": `package fixture

func Sum(n int) int {
	total := 0
	for i := range n {
		total += i
	}
	return total
}
`,
		"fixture_test.go": `package fixture

import "testing"

func TestSum(t *testing.T) {
	if got := Sum(3); got != 3 {
		t.Fatalf("Sum(3) = %d, want 3", got)
	}
}
`,
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0600))
	}
	t.Chdir(dir)

	for _, mode := range []string{"set", "count", "atomic"} {
		t.Run(mode, func(t *testing.T) {
			profile := filepath.Join(dir, mode+".cov")
			cmd := exec.Command("go", "test", "-covermode="+mode, "-coverprofile="+profile, ".")
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "%s", output)

			data, err := ConvertProfiles(profile)
			require.NoError(t, err)
			var result struct{ Packages []*gocov.Package }
			require.NoError(t, json.Unmarshal(data, &result))
			require.Len(t, result.Packages, 1)
			pkg := result.Packages[0]
			assert.Equal(t, "example.com/coveragefixture", pkg.Name)
			require.Len(t, pkg.Functions, 1)
			function := pkg.Functions[0]
			assert.Equal(t, "Sum", function.Name)
			assert.Equal(t, filepath.Join(dir, "fixture.go"), function.File)
			require.Len(t, function.Statements, 4)
			expectedCounts := []int64{1, 1, 1, 1}
			if mode != "set" {
				expectedCounts[2] = 3
			}
			for i, statement := range function.Statements {
				assert.Equal(t, expectedCounts[i], statement.Reached)
			}
		})
	}
}

func TestExprName(t *testing.T) {
	source := `
package foo

func Function() {}
func (x Foo) Method() {}
func (x *Foo) PtrMethod() {}
func (x *Foo[T]) GenericMethod() {}
`

	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "", source, 0)
	if err != nil {
		t.Fatal(err)
	}

	function0 := parsed.Decls[0].(*ast.FuncDecl)
	assert.Equal(t, "Function", functionName(function0))

	function1 := parsed.Decls[1].(*ast.FuncDecl)
	assert.Equal(t, "Foo.Method", functionName(function1))

	function2 := parsed.Decls[2].(*ast.FuncDecl)
	assert.Equal(t, "Foo.PtrMethod", functionName(function2))

	function3 := parsed.Decls[3].(*ast.FuncDecl)
	assert.Equal(t, "Foo[T].GenericMethod", functionName(function3))

}
