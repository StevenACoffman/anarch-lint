---
format: 3
---
Source: Simple Made Easy, Rich Hickey (Strange Loop keynote, September 2011, transcript)
Scope:  Language-agnostic design of software constructs and systems, with examples drawn from Clojure, Haskell, Java, C++, C#, Common Lisp, Scheme, SQL, LINQ, Datalog and Prolog; covers choosing language features and libraries, mutable state versus immutable values, representing information as data, interface and protocol design, polymorphism, component composition and dependency injection, inter-component communication via queues, placement of business policy, and the design habits used to judge simplicity versus ease
Limitations: The source is a conference talk, not a design manual: it gives no metrics or thresholds for how small an interface must be, how many components are too few, or when a direct call is acceptable instead of a queue; it explicitly defers the process of simplifying existing code ("a whole separate talk") and offers no solution for environmental complexity such as thread-pool and memory sizing; it calls declarative rule systems and integrated declarative data manipulation a "cutting edge" area with few good tools; it does not address performance, security, observability, deployment or team process; it exempts "really short term" work without defining short term; its dismissal of sophisticated type systems is a joke rather than an argument; and it acknowledges there are reasons to depart from its list of simpler constructs without saying what they are

§1.1  [MUST][METHOD]  Judge a language feature, library or design by the entanglement in what it produces at runtime and over the program's life, not by how short, pleasant or quick it is to write; a construct that saves keystrokes but interleaves concerns is the worse choice.
      Users run and maintain the artifact, not the source text, so a construct chosen for authoring convenience leaves its complexity in every later change, bug hunt and requirement — the "knitted castle" that tests and refactoring cannot make cheap to change.
      ✗  Adopt an ORM because "I only had to type 16 characters" to load an object graph
      ✓  Compare the ORM and plain SQL over data by what each couples together (logic to representation, identity to value) and choose on that
      ↦  §Construct vs Artifact: "We have to start assessing our constructs based around the artifacts, not around the look and feel of the experience of typing it in or the cultural aspects of that."

§1.2  [MUST][METHOD]  Do not call a construct or design simple because it is familiar, already installed, or readable to the current team, and do not reject one as complex because it is unfamiliar; assess simplicity only by whether its parts are interleaved.
      Simplicity is objective (one can look for the braids) while ease is relative to the reader, so conflating them lets familiar but entangled tools win every argument and blocks learning anything significantly different from what is already known.
      ✗  "Mutable objects are simpler than immutable maps — every developer here already knows how to use them"
      ✓  "Mutable objects are familiar to us but complect value and time; immutable maps are unfamiliar but not interleaved — the unfamiliarity is a learning cost we can pay"
      ↦  §What's in your Toolkit?: "Simple does not mean I already know what it means."

§1.3  [MUST][METHOD]  Do not accept a passing test suite, a type checker, or refactoring tools as justification for an entangled design; require the design itself to be simple enough to reason about informally.
      Every bug found in production passed the type checker and all the tests, so the only remaining tool is reasoning about where the fault can and cannot be, and entanglement is what destroys that ability (overrides common practice — apply as stated).
      ✗  "This module is tangled, but it has 95% coverage, so we can change it safely"
      ✓  "Coverage is a safety net; first separate the parts so a change to pricing cannot affect persistence, then rely on the tests"
      ↦  §Simplicity is a Choice: "All the reliability tools you have, right, since they're not about simplicity, they're all secondary."

§1.4  [SHOULD][METHOD]  Start from the simpler construct in each pair (values over state, functions over methods, data over syntax, queues over actors, declarative data manipulation over ORM, polymorphism à la carte over inheritance and switching), and require a stated reason before departing from it.
      Defaulting to the familiar complex construct reinforces a "culture of complexity" in which entanglement is adopted without anyone deciding to accept it, whereas a recorded reason makes each departure a deliberate tradeoff.
      ✗  Model a new feature with a mutable class hierarchy because that is how the rest of the codebase does it
      ✓  Model it with immutable data and plain functions, and use a mutable reference only where a recorded requirement needs identity over time
      ↦  §The Simplicity Toolkit: "There are reasons why you might have to get off of this list, but, boy, there's no reason why you shouldn't start with it."

§1.5  [SHOULD][METHOD]  Do not count the number of components, functions or operations as a measure of complexity; prefer a design with more independent parts over one with fewer parts that are tied together.
      Treating "fewer things" as "simpler" pushes designs toward merging independent concerns, and the merged knot loses the independence of decisions (location, performance, policy) that makes change cheap.
      ✗  Reject a design that splits a job into five small components in favour of one component that does all five jobs
      ✓  Accept the five components as long as none of them knows about the internals of another
      ↦  §Simple: "What matters for simplicity is that there is no interleaving, not that there's only one thing"

§1.6  [SHOULD][METHOD]  When proposing or evaluating a new tool, library or language feature, state the complexity it introduces alongside its benefits.
      Evaluations that list only benefits import incidental complexity unnoticed, and every incidental ball juggled leaves less capacity for the problem's own complexity.
      ✗  "Let's adopt this framework — it is shorter and has great DX"
      ✓  "This framework is shorter, but it couples our handlers to its lifecycle and ties logic to its object model; here is what that costs us"
      ↦  §Alan Perlis quote: "we're not looking carefully enough at the byproducts."

§1.7  [SHOULD][METHOD]  Before implementing anything other than short-lived throwaway code, spend design time separating the problem's concerns, and do not skip that step to start typing sooner.
      Focusing on ease gives the fastest start but complexity accumulates until most iterations redo earlier work, while the up-front simplification is what lets speed be sustained; the source exempts only "really short term" work.
      ✗  Start coding a long-lived billing service the day it is assigned, deferring design to later sprints
      ✓  First identify which parts are policy, which are data, which are operations and which are scheduling, then start implementing them separately
      ↦  §Simplicity Made Easy: "We're going to spend a little time upfront simplifying things before we get started."

§1.8  [SHOULD][ARCH]  Do not accept separate classes, modules or layers as evidence that a design is simple; check what each component is allowed to know about the others, and treat hiding implementation details as distinct from abstraction.
      Partitioning and stratification can be applied equally to complected components, which then fail in the field when one silently relies on the other's behaviour; neither layering nor information hiding removes the braid.
      ✗  "It's clean — the service, repository and controller are in separate packages" while the controller depends on the repository returning rows in insertion order
      ✓  "Each layer depends only on the other's declared interface; no layer relies on behaviour the interface does not state"
      ↦  §Modularity and Simplicity: "be particularly careful not to be fooled by code organization."

§1.9  [CONSIDER][METHOD]  When decomposing a feature, separate it along who (entities and subcomponents), what (operations), how (implementation), when and where (scheduling and location), and why (policy and rules), and treat any piece that answers two of these as a candidate for splitting.
      Asking each question separately exposes concerns that would otherwise be decided together in one place; the source offers it as "one approach", not a required method.
      ✗  One `OrderProcessor` class that defines the operations, implements them, decides which warehouse handles each order, and encodes the discount policy
      ✓  An `Orders` interface (what), an implementation (how), a queue between intake and fulfilment (when/where), and a separate discount rule set (why)
      ↦  §Abstraction for Simplicity: "one approach you can take is just to do who, what, when, where, why, and how."

§2.1  [MUST][CODE]  Do not treat hiding a mutable variable inside a method or object as containing its complexity; if a call can return different results for the same arguments, treat every caller as stateful too, and only consider the state contained when the enclosing component presents a same-input-same-output interface.
      State complects value and time and leaks through every wrapper that is itself stateful, so a reviewer who trusts encapsulation will miss that reproducing a field failure requires recreating the whole state the client was in.
      ✗  `cache.Get(key)` returns a different value after another caller's `Refresh()`, and callers are reviewed as if `Get` were pure because the map is private
      ✓  `Price(order, rates)` is computed from its arguments alone, with the mutable rate store read once at the boundary and passed in as a value
      ↦  §State is Never Simple: "The only time you can really, you know, get rid of it is when you put it inside something that's able to present a functional interface on the outside, a true functional interface: same input, same output."

§2.2  [MUST][ARCH]  When a design requires consistency across several pieces of data, use transactions over immutable values rather than coordinating updates to mutable fields by hand.
      Hand-coordinated mutation lets readers observe partially updated combinations, which is inconsistency — several things standing apart that the programmer must still reason about together.
      ✗  Update `account.balance` and then `ledger.entries` in two separate mutations, relying on call order for correctness
      ✓  Compute the new balance and ledger as values and commit both in one transaction
      ↦  §The Simplicity Toolkit: "If you want consistency, you need to use transactions, and you need to use values."

§2.3  [SHOULD][CODE]  Represent information with immutable values — declared immutable (`final`, `val`, `const`) and, for aggregates, persistent collections — instead of mutable objects, and make immutability the default with mutability opted into explicitly.
      Mutable objects complect state, identity and value so no reader can obtain a stable value, and codebases where mutability must be requested end up with far less state because most of it was never needed.
      ✗  `user.setEmail(e)` mutating a shared `User` instance that other code holds references to
      ✓  `updated := withEmail(user, e)` returning a new immutable value, with aggregates held in a persistent map or vector
      ↦  §What's in your Toolkit?: "state and objects are complex, and values are simple and can replace them in many cases"

§2.4  [SHOULD][CODE]  When mutable state is unavoidable, use a reference construct that yields an immutable snapshot value, and pass that value — not the reference or variable — to downstream code.
      Passing the reference means every downstream function re-reads a changing variable and becomes stateful itself, whereas extracting a value is the path back to simple code for the rest of the program.
      ✗  `render(configRef)` where `render` dereferences `configRef` several times during one call
      ✓  `cfg := configRef.Load(); render(cfg)` where `cfg` is an immutable value
      ↦  §Not all refs/vars are Equal: "If I have to pass that variable to somebody else or a reference to something that's going to find the variable every time through the varying thing, I'm poisoning the rest of my system."

§2.5  [SHOULD][ARCH]  Do not choose an eventually consistent store or replication scheme for convenience; adopt it only when a stated requirement rules out a consistent design, and record the reasoning cost it imposes.
      Inconsistency forces programmers to reason about several things that stand apart as if they stood together, which the source calls almost definitionally complex.
      ✗  Pick an eventually consistent document store for a single-region order system because its client library is easier to set up
      ✓  Use a transactional store, and choose eventual consistency only after documenting the availability or latency requirement that forces it
      ↦  §What's in your Toolkit?: "And inconsistency is very complex."

§3.1  [MUST][CODE]  Do not wrap information in objects that hide it behind methods; reserve objects for encapsulating things the program cannot touch directly, such as I/O devices and external resources.
      Encapsulated information cannot be processed by generic data functions, so every new class needs bespoke code to read, transform and serialise it instead of reusing code that was proven correct once.
      ✗  `class Address { private street; getStreet(); setStreet(); formatForLabel() }` used to carry a customer's address between layers
      ✓  An address as a plain map or record `{street, city, postcode}` passed to generic functions, with an object used only for the printer connection
      ↦  §Information is Simple: "They were never supposed to be applied to information."

§3.2  [SHOULD][CODE]  Use generic collections (maps, sets, sequences) directly for information instead of defining a new named class each time a new piece of information appears (overrides common practice — apply as stated).
      Hundreds of bespoke data types that differ from maps, sets and sequences only in name make it hard to write programs that manipulate the data itself, and each type needs its own accessors, equality and conversion code.
      ✗  Define `PatientVisitSummaryDTO` with ten getters because a report needs ten fields
      ✓  Build the report from a map with those ten keys and use existing map, filter and select functions on it
      ↦  §Information is Simple: "Please start using maps and sets directly."

§3.3  [SHOULD][ARCH]  Exchange information between programs and components as structured data, not as free-form strings that each receiver must parse with bespoke code.
      Arbitrary text in and out means every consumer writes and maintains its own parser, and every change of format breaks consumers silently.
      ✗  A tool that emits `"user=alice; roles: admin,ops (since 2020)"` for other tools to regex apart
      ✓  A tool that emits `{"user": "alice", "roles": ["admin", "ops"], "since": 2020}`
      ↦  §The Simplicity Toolkit: "So we can get rid of that. Just use data."

§3.4  [SHOULD][ARCH]  Prefer declarative data manipulation (SQL, LINQ, Datalog) over object-relational mapping for accessing stored data.
      An ORM ties application logic to its representational classes, so changing storage shape or reusing the logic elsewhere requires changing both together.
      ✗  Business logic navigating `order.getCustomer().getAccounts()` through lazily loaded ORM entities
      ✓  A SQL query that returns the needed rows as plain data, consumed by logic that knows nothing about tables or entity classes
      ↦  §Information is Simple: "it will tie your logic to representational things"

§3.5  [SHOULD][CODE]  When designing a configuration format, query interface or DSL, express it as data structures rather than a new textual syntax.
      Syntax complects meaning with order and requires a parser, while data can be generated, inspected and transformed by ordinary code.
      ✗  A routing DSL parsed from strings like `"GET /users/:id -> showUser if admin"`
      ✓  Routes as data: `[{method: "GET", path: "/users/:id", handler: showUser, guard: isAdmin}]`
      ↦  §The Complexity Toolkit: "It's inferior to data in every way."

§4.1  [SHOULD][CODE]  Keep each interface, protocol or type class to a few closely related functions, and let a function that needs several capabilities take several small abstractions rather than one combined one.
      Large interfaces bundle unrelated capabilities so implementations and callers cannot be separated, and a program built on them is much harder to break apart later.
      ✗  `interface Repository { Get; Put; Delete; List; Search; Export; Audit; Migrate }` required by a function that only reads
      ✓  `func Report(r Reader, s Searcher)` with `Reader { Get }` and `Searcher { Search }` as separate interfaces
      ↦  §What: "they should be really small, much smaller than what we typically see."

§4.2  [SHOULD][CODE]  Define abstractions only in terms of values and other abstractions: no concrete implementation types in an interface's parameters or return types.
      A concrete type in an abstraction's signature fixes how it is implemented and couples every implementer and caller to that type.
      ✗  `interface Store { Save(conn *PostgresConn, u *UserEntity) }`
      ✓  `interface Store { Save(u User) error }` where `User` is a plain value
      ↦  §What: "They should only use values and other abstractions in their definitions."

§4.3  [SHOULD][CODE]  Do not let an abstraction's name, signature or stated semantics dictate how it is carried out, such as a required evaluation order, a concrete strategy, or a specific engine.
      An abstraction that implies how ties the implementer's hands, so the work cannot be handed to a database engine, logic engine or alternative implementation.
      ✗  `interface Matcher { LinearScanFirstMatch(items) }` whose contract requires scanning left to right
      ✓  `interface Matcher { Matching(items) Set }` leaving order and strategy to the implementation
      ↦  §How: "beware of abstractions that dictate how in some subtle way"

§4.4  [SHOULD][CODE]  When the order of processing is irrelevant to the result, use set or collection functions (`map`, `filter`, set operations, declarative queries) rather than imperative loops or `fold`.
      Loops complect what is computed with how it is computed, and `fold` still imposes a left-to-right order that the problem did not require, which prevents reordering or parallelising later.
      ✗  `for i := 0; i < len(xs); i++ { if xs[i].Active { out = append(out, xs[i].ID) } }` to collect IDs whose order nobody uses
      ✓  `ids := set(map(id, filter(active, xs)))`
      ↦  §What's in your Toolkit?: "Imperative loops, fold even, which seems kind of higher level, still has some implications that tie two things together, whereas set functions are simpler."

§4.5  [SHOULD][CODE]  Do not overload one construct, field or notation to carry several unrelated roles; give each role its own construct.
      Overloading forces every reader to decide which role is meant at each use, which is complexity by definition even after the notation becomes familiar.
      ✗  A `status` integer that holds a lifecycle stage when positive and an error code when negative
      ✓  Separate `stage` and `error` fields
      ↦  §Parens are Hard!: "that overloading is a form of complexity by the definition"

§5.1  [MUST][ARCH]  Do not let a component depend on behaviour of another component that is not stated in the abstraction between them.
      Hidden assumptions make two separately organised components complected, and the dependency surfaces only in the field when the other side legitimately does something new.
      ✗  A consumer that breaks if the pricing service ever returns `17` because it uses that value as an internal sentinel
      ✓  A consumer that relies only on the pricing interface's documented contract, with any sentinel made an explicit part of that contract
      ↦  §Modularity and Simplicity: "This thing presumes that that thing never returns the number 17."

§5.2  [SHOULD][CODE]  Accept subcomponents as arguments instead of constructing or hard-wiring them inside the component.
      Hard-wired subcomponents fix the component's composition, so a different implementation, policy or test double requires editing the component itself.
      ✗  `func NewSync() *Sync { return &Sync{store: NewS3Store(), clock: time.Now} }`
      ✓  `func NewSync(store Store, now func() time.Time) *Sync`
      ↦  §Who: "You don't want to, like, hardwire what the subcomponents are."

§5.3  [SHOULD][ARCH]  Split policy decisions out of the components that do the work into their own subcomponents, and prefer more, smaller subcomponents over a few large ones.
      Policy embedded in a worker cannot be changed without changing the worker, and most components have no subcomponents at all, which leaves every decision tangled in one place.
      ✗  A `Retrier` whose backoff schedule, retry limit and retryable-error list are hard-coded inside its loop
      ✓  A `Retrier` that takes a `RetryPolicy` subcomponent deciding whether and when to retry
      ↦  §Who: "split out policy and stuff like that."

§5.4  [SHOULD][CODE]  Connect implementations to the operations they serve through open polymorphism (interfaces, protocols, type classes, multimethods) rather than `switch` statements or pattern matching over types (overrides common practice — apply as stated).
      A `switch` or `match` gathers every pairing of who-does-it with what-happens in one closed place, so adding a new kind requires editing that central code instead of supplying a new implementation.
      ✗  `switch shape.(type) { case Circle: ...; case Square: ... }` inside `Area()`
      ✓  An `Area()` method on an `Areaer` interface, implemented separately by `Circle` and `Square`
      ↦  §How: "You strictly want to connect these things together using those polymorphism constructs."

§5.5  [SHOULD][CODE]  Do not use inheritance to share behaviour or define types; compose independent types with interfaces or protocols instead.
      Inheritance complects the parent and child types, so a change to the parent changes every descendant and the types cannot be understood separately.
      ✗  `class PremiumAccount extends Account` overriding `fee()` and inheriting twelve other methods
      ✓  `Account` as plain data plus a `FeePolicy` interface with a premium implementation
      ↦  §The Complexity Toolkit: "Inheritance complects types."

§5.6  [SHOULD][CODE]  Write operations as free functions grouped in namespaces or modules rather than as methods on stateful classes, and do not use a class merely as a namespace.
      Methods complect function with state and, in some languages, with namespacing, so two inherited methods with the same name collide and each method can depend on hidden instance state.
      ✗  `class StringUtils { trimAll(); slugify(); }` instantiated to call helpers, or `invoice.computeTax()` reading mutable fields
      ✓  `strings.TrimAll(s)` in a `strings` namespace, and `tax.Compute(invoice)` taking an immutable invoice value
      ↦  §The Complexity Toolkit: "Methods complect function and state, ordinarily, right?"

§5.7  [SHOULD][CODE]  Keep implementation code free of everything except doing its job: no embedded policy, scheduling, location or knowledge of its callers.
      An implementation that also decides when, where or why cannot be replaced or reused without disturbing those other decisions.
      ✗  An image-resize function that also decides whether the user's plan allows resizing and enqueues the next pipeline step
      ✓  `Resize(img, size)` that only resizes, with plan checks and pipeline routing done elsewhere
      ↦  §How: "All these implementations should be islands as much as possible."

§5.8  [CONSIDER][CODE]  Where the language supports it, define data types, sets of function specifications, and the connections between them as three independent declarations (polymorphism à la carte, e.g. Clojure protocols, Haskell type classes) rather than binding them in a class definition.
      Binding genericity to a type's definition means a new operation set cannot be attached to an existing type without editing it; the source notes few library solutions exist for languages without this feature.
      ✗  Edit `class Invoice` to add `implements Exportable` and an `export()` method
      ✓  `extend-protocol Exportable Invoice (export [i] ...)` declared alongside the export code, leaving `Invoice` untouched
      ↦  §The Simplicity Toolkit: "the genericity is not tied to anything in particular."

§6.1  [SHOULD][ARCH]  When one component hands work to the next stage and does not need that stage's result to continue, connect them through a queue instead of having the first call the second directly.
      A direct call makes A know where B is and fixes when B runs to whenever A calls it, complecting location and timing into the structure of the program.
      ✗  `IngestHandler` calls `Enricher.Process(rec)` directly after parsing each record
      ✓  `IngestHandler` puts parsed records on a queue that `Enricher` consumes
      ↦  §When, Where: "if thing A calls thing B, you just complected it."

§6.2  [SHOULD][ARCH]  Prefer plain queues (from a library) over actor frameworks or special communication languages for passing work between concurrent parts.
      An actor complects what is to be done with who will do it, so the work cannot be redirected to a different worker without changing the addressing.
      ✗  Send `ResizeImage` messages to a specific named actor `resizer-7`
      ✓  Put `ResizeImage` jobs on a queue from which any resize worker can take them
      ↦  §The Complexity Toolkit: "Actors complect what's going to be done and who is going to do it."

§6.3  [CONSIDER][ARCH]  Gather business policy and rules into one declarative place (a rule system or rule table) rather than scattering conditionals through the program, and do not rely on English-language test scenarios as the place where customers read the rules; the source calls this a "cutting edge" area, so apply it where suitable tooling exists.
      Rules strewn through the program are complected with its structure, so neither a customer nor a developer can see what the application's policy is without reading all of the source.
      ✗  Discount eligibility checks repeated as `if` statements in checkout, invoicing and the admin UI, documented only in Gherkin scenarios
      ✓  A single declarative discount rule set evaluated by checkout, invoicing and the admin UI
      ↦  §Why: "Try to find a declarative system or a rule system that lets you do this work."

§6.4  [CONSIDER][ARCH]  Do not let each component independently size shared resources such as thread pools or memory budgets; route those decisions to the one place that has information about all contenders.
      Policies chosen per component (for example every library sizing its pool to the core count) do not compose and oversubscribe the machine; the source has no general solution, so apply conservatively.
      ✗  Three libraries in one process each create a thread pool sized to `runtime.NumCPU()`
      ✓  The application decides one concurrency budget and passes each library its share
      ↦  §Environmental Complexity: "that's a decision that needs to be made by someone who has better information."
