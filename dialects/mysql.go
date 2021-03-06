package dialects

import (
	"fmt"

	"github.com/outdoorsy/gorp"
)

type MySQLDialect struct {
	gorp.MySQLDialect
}

func (dialect MySQLDialect) Limit(bindVar interface{}) string {
	return fmt.Sprintf("limit %s", bindVar)
}
