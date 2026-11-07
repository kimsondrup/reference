package idnahost_test

import (
	"fmt"

	"github.com/distribution/reference"
	"github.com/distribution/reference/idnahost"
)

func ExampleParseNormalizedNamed() {
	named, err := idnahost.ParseNormalizedNamed("Bücher.example:5000/app:v1")
	if err != nil {
		panic(err)
	}
	fmt.Println(reference.Domain(named))
	fmt.Println(named.String())
	// Output:
	// xn--bcher-kva.example:5000
	// xn--bcher-kva.example:5000/app:v1
}
