// Command client runs billing.
package main

import (
	"fmt"

	"example.com/cx/billing"
)

func main() { fmt.Println(billing.Approve(billing.Invoice{})) }
