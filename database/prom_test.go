package database

import (
	"testing"

	"github.com/alecthomas/repr"
)

func TestPromqlQuery(t *testing.T) {

	Connect()
	samp := GetSrcGlobUsage(5, -5, 1000, true)

	repr.Println(samp)

}
