package fieldsjson

import (
	"slices"
	"testing"

	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// TestCreditKeysCoverEveryCredit tests that every credit field has a place in the importance order, once.
func TestCreditKeysCoverEveryCredit(t *testing.T) {
	credits := models.JSONGroupFields[sharedconsts.MetaOpAllCredits]
	if len(creditKeys) != len(credits) {
		t.Errorf("creditKeys has %d fields, the all-credits group has %d", len(creditKeys), len(credits))
	}
	for _, k := range credits {
		if !slices.Contains(creditKeys, k) {
			t.Errorf("creditKeys lacks %q", k)
		}
	}
}
