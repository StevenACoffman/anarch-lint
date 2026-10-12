// Command server serves the web subsystem.
package main

import (
	"fmt"

	"example.com/cx/web"
)

func main() { fmt.Println(web.Admin("admin")) }
