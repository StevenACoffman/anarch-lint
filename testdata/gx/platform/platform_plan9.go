package platform

import (
	"os"
	"strings"
)

// Args joins the arguments.
func Args() string { return strings.Join(os.Args, " ") }
