package connascence

import "github.com/StevenACoffman/anarch-lint/internal/arch"

// assumptions lists the choices behind the connascence measures: how the
// axes are defined, and the fixed inputs of the detectors.
func assumptions() []arch.Assumption {
	return append(axisAssumptions(), detectorAssumptions()...)
}

// axisAssumptions are the definitions of strength, locality and degree, and
// of how facts and candidates gate.
func axisAssumptions() []arch.Assumption {
	return []arch.Assumption{
		{
			ID: "strength-order",
			Text: "Strengths follow connascence.io: name 1, type 2, meaning 3, position 4, algorithm 5, " +
				"execution 6, timing 7, value 8, identity 9. coupling.dev ranks position above " +
				"algorithm; only a maxStrength limit of 4 would decide differently.",
			Source: "connascence.md §1.1, §9.1",
		},
		{
			ID: "default-limits",
			Text: "maxStrength defaults to 9 up to the package, 5 across subsystems, 3 across a module " +
				"and 2 beyond it; no degree limit applies unless the model sets one. Borrowed " +
				"guidance from connascence.io's locality principle, not a formula.",
			Source: "connascence.md §1.4",
		},
		{
			ID: "locality",
			Text: "arch.Locality is the first of function, type, file, package, subsystem, system (one Go " +
				"module) and external that holds every site; degree counts the distinct entities " +
				"one level below it.",
			Source: "connascence.md §1.2, §1.3",
		},
		{
			ID: "degree-ratchet",
			Text: "check ratchets the degree of each fact of meaning or stronger at a subsystem " +
				"boundary or beyond; the degree of name and type may grow, as ordinary use of an " +
				"exported name does.",
			Source: "connascence.md §2.1, §6",
		},
		{
			ID: "facts-and-candidates",
			Text: "Facts gate; candidates never do until an approved verdict confirms them, and an " +
				"approved rejection sets them aside until their evidence changes.",
			Source: "connascence.md §5",
		},
	}
}

// detectorAssumptions are the fixed inputs of the detectors: none is a model
// knob, and each becomes one only once a user needs it.
func detectorAssumptions() []arch.Assumption {
	return []arch.Assumption{
		{
			ID: "excluded-values",
			Text: "The values 0, 1, -1 and \"\" carry no meaning of their own and are left out of the " +
				"value detectors; arguments to fmt, errors, log, log/slog and testing are messages " +
				"and are left out too.",
			Source: "connascence.md §2.3",
		},
		{
			ID: "parameter-threshold",
			Text: "A parameter list of 4 or more is connascence of position; runs of two or more " +
				"adjacent parameters of one basic type are candidates of meaning.",
			Source: "connascence.md §2.3, §2.4",
		},
		{
			ID:     "lifecycle-pairs",
			Text:   "Lifecycle pairs are Open/Close, Start/Stop, Connect/Disconnect and Begin/Commit.",
			Source: "connascence.md §3.1",
		},
		{
			ID: "registrations",
			Text: "Registration functions are net/http's Handle and HandleFunc and the flag package's " +
				"definitions, keyed by their route or flag name; two calls may use different " +
				"instances, so collisions are candidates.",
			Source: "connascence.md §3.5",
		},
		{
			ID: "read-only-variables",
			Text: "An exported variable that no function writes, such as a sentinel error, is " +
				"connascence of identity only as a candidate: it behaves as a constant.",
			Source: "connascence.md §3.4",
		},
		{
			ID: "clones",
			Text: "Clones are function bodies of 10 or more statements with the same shape once names " +
				"and literal values are removed; a rule re-implemented in different code is missed.",
			Source: "connascence.md §2.5, §9.2",
		},
		{
			ID: "partial-clones",
			Text: "Partial clones are github.com/golangci/dupl's: runs of statements, each of at " +
				"least 60 syntax-tree nodes, repeated in two or more packages, found with facts " +
				"only. Copies that are whole functions of one shape are left to the clones detector.",
			Source: "connascence.md §2.5; IMPROVEMENTS.md 9.1",
		},
		{
			ID:     "site-cap",
			Text:   "An instance lists its first 20 sites in sorted order; siteCount gives the total.",
			Source: "connascence.md §6",
		},
	}
}
