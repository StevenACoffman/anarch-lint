package connascence_test

import (
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
	"github.com/StevenACoffman/anarch-lint/internal/connascence"
)

// docs/adr/0002: the gate ratchets the degree of meaning and stronger at a
// subsystem boundary or beyond, and lets name and type grow.
func TestADR_0002_DegreeRatchetSkipsNameAndType(t *testing.T) {
	t.Parallel()
	instance := func(fp string, kind arch.Strength, at arch.Locality) arch.Connascence {
		return arch.Connascence{Fingerprint: fp, Kind: kind, Locality: at, Degree: 3}
	}
	r := connascence.Report{Instances: []arch.Connascence{
		instance("name", arch.StrengthName, arch.LocalitySystem),
		instance("type", arch.StrengthType, arch.LocalitySubsystem),
		instance("meaning-in-package", arch.StrengthMeaning, arch.LocalityPackage),
		instance("meaning", arch.StrengthMeaning, arch.LocalitySubsystem),
		instance("algorithm", arch.StrengthAlgorithm, arch.LocalityExternal),
	}}
	var in arch.GateInput
	r.AddTo(&in)
	var ratcheted []string
	for _, m := range in.Metrics {
		ratcheted = append(ratcheted, m.Name)
	}
	equals(t, []string{"connascence.degree/algorithm", "connascence.degree/meaning"}, ratcheted)
}
