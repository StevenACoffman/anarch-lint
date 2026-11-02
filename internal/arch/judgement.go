package arch

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
)

// Slot statuses.
const (
	// SlotPending is a slot no verdict addresses.
	SlotPending SlotStatus = "pending"
	// SlotSupplied is a slot filled by an approved verdict for the current
	// evidence.
	SlotSupplied SlotStatus = "supplied"
	// SlotStale is a slot whose verdict judged different evidence: the code
	// changed since, so the verdict no longer applies.
	SlotStale SlotStatus = "stale"
	// SlotUnapproved is a slot whose verdict no human has attested yet, such
	// as a judge's draft.
	SlotUnapproved SlotStatus = "unapproved"
	// SlotUnresolved is a slot whose approved verdict found the evidence
	// insufficient to answer. It counts as no answer: it neither gates nor
	// suppresses, and an MMI criterion stays unscored.
	SlotUnresolved SlotStatus = "unresolved"
)

// AnswerInsufficient is the answer every slot accepts besides its own: the
// evidence cannot settle the question. An MMI verdict gives it instead of a
// score. Being able to say so keeps a judge from guessing.
const AnswerInsufficient = "insufficient-evidence"

// Causes of a stale slot.
const (
	// StaleQuestion is a verdict given under an earlier edition of the
	// question: its words, answers or revision changed.
	StaleQuestion StaleCause = "question"
	// StaleEvidence is a verdict that judged different evidence.
	StaleEvidence StaleCause = "evidence"
)

// SlotStatus says whether a judgement slot has a verdict that applies.
type SlotStatus string

// StaleCause says why a verdict no longer applies to its slot.
type StaleCause string

// JudgementSlot is a question waiting for a reviewer: a human expert or an
// adversarial LLM (mmi.md §7.6). It carries the question, the deterministic
// evidence a reviewer should weigh, and a hash of that evidence. A verdict
// applies only to the evidence it was given, so a code change that alters the
// evidence makes the verdict stale instead of silently reusing it.
//
// A slot is either an MMI criterion, such as "1.1.5", answered with a score,
// or a member of a family, such as "subdomain/billing", answered with one of
// Answers (connascence.md §5, §9). A slot never gates by itself: an MMI
// verdict narrows the MMI interval, and an approved family verdict can turn a
// candidate into a finding or suppress one.
type JudgementSlot struct {
	// Criterion is the slot's ID: an MMI criterion, or "family/subject".
	Criterion string `json:"criterion"`
	Question  string `json:"question"`
	// Answers lists the valid answers of a family slot; an MMI slot takes a
	// score from 0 to 10 instead.
	Answers []string `json:"answers,omitempty"`
	// Edition identifies the question's wording, answers and revision; a
	// verdict given under another edition is stale.
	Edition  string   `json:"edition"`
	Evidence []string `json:"evidence"`
	// Context is what a reviewer weighs but a judge must not see: what the
	// answer would set off, such as the limit a candidate exceeds. Showing
	// it would tell a judge which answer gates (mmi-plan.md §9.3 B).
	Context []string `json:"context,omitempty"`
	// Undecided says why the deterministic measure could not settle the
	// slot, so a judge knows what is left to decide (mmi-plan.md §9.3 H).
	Undecided    string     `json:"undecided"`
	EvidenceHash string     `json:"evidenceHash"`
	Status       SlotStatus `json:"status"`
	// StaleBecause says why a stale slot's verdict no longer applies.
	StaleBecause StaleCause `json:"staleBecause,omitempty"`
	Verdict      *Verdict   `json:"verdict,omitempty"`
}

// SlotRequest is what a measure asks a reviewer: the slot's criterion, the
// evidence a judge may see, and the context only a human sees. Undecided,
// when set, replaces the family's reason that no measure could decide, with
// one particular to this slot.
type SlotRequest struct {
	Criterion string
	Evidence  []string
	Context   []string
	Undecided string
}

// JudgeView is all a judge may see of a slot: the question, the answers and
// the evidence, each piece numbered so a reply can cite it. It has no field
// for the context, the status, an earlier verdict or its rationale, so none
// can reach a judge (gnosis's blinded critic projection).
type JudgeView struct {
	ID        string    `json:"id"`
	Question  string    `json:"question"`
	Answers   []string  `json:"answers,omitempty"`
	Undecided string    `json:"undecided"`
	Exhibits  []Exhibit `json:"exhibits"`
}

// Exhibit is one numbered piece of evidence: E1, E2, ….
type Exhibit struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// question is what a slot asks, and the answers it accepts: nil for an MMI
// criterion, which takes a score. revision is turned by hand when the
// question's meaning changes without its words, so its verdicts go stale.
// undecided is why no deterministic measure answers it.
type question struct {
	text      string
	answers   []string
	revision  int
	undecided string
}

// NewSlot returns the slot req asks for, with its hash, filled by whichever
// verdict applies.
//
// Requires: req.Criterion is an MMI judgement criterion or "family/subject"
// of a known family.
func NewSlot(req *SlotRequest, verdicts []Verdict) JudgementSlot {
	q, _ := questionOf(req.Criterion)
	slot := JudgementSlot{
		Criterion: req.Criterion,
		Question:  q.text,
		Answers:   q.answers,
		Edition:   q.edition(),
		Evidence:  req.Evidence,
		Context:   req.Context,
		Undecided: cmp.Or(req.Undecided, q.undecided),
		Status:    SlotPending,
	}
	slot.EvidenceHash = slot.hash()
	for i := range verdicts {
		v := verdicts[i]
		if v.Criterion != req.Criterion {
			continue
		}
		slot.Verdict = &v
		switch {
		case v.EvidenceHash != slot.EvidenceHash:
			slot.Status, slot.StaleBecause = SlotStale, StaleEvidence
			if v.Edition != "" && v.Edition != slot.Edition {
				slot.StaleBecause = StaleQuestion
			}
		case !v.attestedBy(RoleHuman):
			// Every family is judged by humans until canaries can qualify
			// a judge (mmi-plan.md §9.3 D, E), so a judge's verdict is a
			// draft.
			slot.Status = SlotUnapproved
		case v.Answer == AnswerInsufficient:
			slot.Status = SlotUnresolved
		default:
			slot.Status = SlotSupplied
		}
	}
	return slot
}

// edition is the digest of the question's revision, text and answers, or
// its score scale for an MMI criterion.
func (q *question) edition() string {
	scale := q.answers
	if scale == nil {
		scale = []string{"score 0–10"}
	}
	return digest([]string{strconv.Itoa(q.revision), q.text}, scale)
}

// digest is the SHA-256 of groups of strings, each group prefixed by its
// length and each string written as a netstring ("5:hello,"). The encoding
// is injective, so no string can imitate a boundary between two others.
func digest(groups ...[]string) string {
	var b strings.Builder
	for _, g := range groups {
		b.WriteString(strconv.Itoa(len(g)) + ":")
		for _, s := range g {
			b.WriteString(strconv.Itoa(len(s)) + ":" + s + ",")
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// Answer returns the answer of an approved verdict that judged the slot's
// current evidence, and "" for a pending, stale or unapproved slot: the only
// verdicts a measure may act on.
func (slot *JudgementSlot) Answer() string {
	if slot.Status != SlotSupplied {
		return ""
	}
	return slot.Verdict.Answer
}

// JudgeView returns what a judge may see of the slot.
func (slot *JudgementSlot) JudgeView() JudgeView {
	v := JudgeView{
		ID: slot.Criterion, Question: slot.Question, Answers: slot.Answers,
		Undecided: slot.Undecided, Exhibits: make([]Exhibit, len(slot.Evidence)),
	}
	for i, e := range slot.Evidence {
		v.Exhibits[i] = Exhibit{ID: "E" + strconv.Itoa(i+1), Text: e}
	}
	return v
}

// hash is the digest of what the slot asks and shows: its edition,
// criterion, evidence, context and why it is undecided. The context is
// hashed too: a change in what an answer would set off is a reason to judge
// again.
func (slot *JudgementSlot) hash() string {
	return digest(
		[]string{slot.Edition, slot.Criterion, slot.Undecided}, slot.Evidence, slot.Context,
	)
}

// apply scores a judgement criterion from its slot, when the slot is supplied.
func (slot *JudgementSlot) apply(c *Criterion) {
	switch slot.Status {
	case SlotSupplied:
		score := slot.Verdict.Score
		c.Score = &score
		c.Reason = "scored by a verdict a human attested"
	case SlotStale:
		c.Reason = "the verdict judged different evidence; re-review the slot"
		if slot.StaleBecause == StaleQuestion {
			c.Reason = "the verdict answered an earlier edition of the question; re-review the slot"
		}
	case SlotUnapproved:
		c.Reason = "the verdict awaits approval"
	case SlotUnresolved:
		c.Reason = "a reviewer found the evidence insufficient to score it"
	case SlotPending:
		c.Reason = "needs a reviewer; see the judgement slot"
	}
}

// SlotAnswers returns the answers a slot ID accepts, nil for an MMI
// criterion, which takes a score, and false when the ID is no slot.
func SlotAnswers(id string) ([]string, bool) {
	q, ok := questionOf(id)
	return q.answers, ok
}

// questionOf returns the question of a slot ID: an MMI judgement criterion,
// or "family/subject" for one of the families of connascence.md §5 and §9.
func questionOf(id string) (question, bool) {
	if family, subject, ok := strings.Cut(id, "/"); ok {
		q, known := familyQuestion(family)
		q.answers = append(slices.Clone(q.answers), AnswerInsufficient)
		return q, known && subject != ""
	}
	text := mmiQuestion(id)
	return question{
		text: text, revision: 1,
		undecided: "the MMI leaves this criterion to a reviewer (mmi.md §7.6): " +
			"it asks what the structure means, which no measure of the structure answers",
	}, text != ""
}

// familyQuestion returns the question a slot family asks of its subject.
func familyQuestion(family string) (question, bool) {
	confirm := []string{"confirmed", "rejected"}
	q, ok := map[string]question{
		"connascence": {
			text:      "Must these places change together, so that the candidate is connascence?",
			answers:   confirm,
			revision:  1,
			undecided: "a detector found the shape of connascence, not that the places must change together",
		},
		"subdomain": {
			text:      "Is this subsystem a core, supporting or generic subdomain of the business?",
			answers:   []string{"core", "generic", "supporting"},
			revision:  1,
			undecided: "how often the business need changes is a business judgement; churn is only evidence of it",
		},
		"bounded-context": {
			text:      "Is this subsystem a bounded context, with a model of its own?",
			answers:   []string{"no", "yes"},
			revision:  1,
			undecided: "whether a subsystem has a model of its own is a design judgement no import shows",
		},
		"integration-pattern": {
			text: "Which DDD integration pattern do these two subsystems follow?",
			answers: []string{
				"anti-corruption-layer", "conformist", "customer-supplier",
				"open-host-service", "partnership", "separate-ways", "shared-kernel",
			},
			revision:  1,
			undecided: "an integration pattern is an agreement between teams, which imports do not show",
		},
		"contract-type": {
			text:      "Is this exported type part of the domain model or an integration contract?",
			answers:   []string{"contract", "model"},
			revision:  1,
			undecided: "the type plays no contract role, yet another subsystem uses it",
		},
		"async": {
			text:      "Does this package integrate asynchronously, through the messaging it imports?",
			answers:   []string{"async", "sync"},
			revision:  1,
			undecided: "importing a messaging client does not show that the package integrates through it",
		},
		"balance": {
			text:      "Is this Balanced Coupling candidate real?",
			answers:   confirm,
			revision:  1,
			undecided: "the coupling rests on candidate evidence, which a detector found but cannot confirm",
		},
	}[family]
	return q, ok
}

// mmiQuestion returns the reviewer's question for an MMI judgement criterion,
// from the MMI chapter and mmi.md §7.6, or "" for any other criterion.
func mmiQuestion(criterion string) string {
	return map[string]string{
		"1.1.5": "Does each subsystem, layer, package and type have one clear responsibility, " +
			"and does its name describe it?",
		"1.1.6": "Do the packages map the technical layers and the subsystems (domain modules) " +
			"of the target architecture, and is that the right mapping?",
		"1.2.2": "Is each subsystem's internal interface mapped through packages that other " +
			"subsystems can see and use?",
		"3.3": "Are the patterns mapped explicitly, through type names, embedding or " +
			"directives, rather than left for readers to infer?",
		"3.4": "Are domain and technical code separated, as DDD, Quasar or hexagonal " +
			"architecture separate them?",
	}[criterion]
}
