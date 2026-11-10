---
format: 3
---
Source: Simplicity Matters, Rich Hickey (RailsConf keynote, April 2012, transcript)
Scope:  Language-agnostic design of code and systems, addressed to a Ruby/Rails audience with examples drawn from Ruby, Clojure, Java-style interfaces, SQL, Prolog, Datalog, XML and JSON; covers judging simplicity versus ease, evaluating libraries and tools, choosing language constructs (values, functions, polymorphism, loops, syntax), the order problem in lists, arguments, records and call chains, representing information as plain maps rather than classes, and subsystem boundaries, queues, ORMs and consistency models
Limitations: The source is a 35-minute conference talk, not a design manual: it lists ten construct pairs but explicitly declines to explain most of them ("I am not in this talk going to break all of these down"), so actors, managed refs, rules and polymorphism à la carte are named without guidance; it gives no thresholds for when positional arguments, a queue, an ORM or eventual consistency become justified beyond "a good reason" or "unless you really have to"; it says subsystems need "general error handling" without saying what that is; it does not cover how to simplify an existing tangled codebase, performance, security, testing practice, deployment or team process beyond a stand-up question; and its "90% of the time" claim about replacing data classes with hashes leaves the remaining 10% undefined

§1.1  [MUST][METHOD]  Judge whether a design, library or construct is simple only by whether its parts are interleaved, never by whether it is familiar, already installed, or approved for use; reject "simple" claims that rest on familiarity.
      Simplicity is objective while ease is relative to each reader, so conflating them lets familiar but entangled choices win every argument, and the entanglement then taxes everyone who must later change the code.
      ✗  "Adding this to the `User` model is simple — it is how we already do everything here"
      ✓  "Adding this to the `User` model is easy for us, but it ties billing to authentication; a separate component is the simpler choice"
      ↦  §Simple: "simple is objective"

§1.2  [MUST][METHOD]  Do not argue that a construct or dependency is simpler because it reduces the work the team has to do now; evaluate it by how entangled the resulting program is.
      Reducing work done now is what pulling in a "hairball" achieves, and the entanglement it brings stays in the program for every later change, while no tool removes it afterwards.
      ✗  "Use this gem — we would not have to write any of this ourselves"
      ✓  "This gem saves writing the feature but couples our models to its callbacks; writing the small piece we need keeps the parts separate"
      ↦  §Agile manifesto quote: "Simplicity is not about you."

§1.3  [MUST][METHOD]  When proposing or evaluating a library, tool or technology, state what it couples into the program alongside the benefit it offers; do not adopt on benefits alone.
      Evaluations that list only benefits (seconds saved, a "great characteristic") import complexity nobody chose, and that complexity is what later stops the team changing the software.
      ✗  "Adopt this framework — it lets us scaffold an endpoint ten seconds faster"
      ✓  "This framework scaffolds endpoints faster but binds our handlers to its object lifecycle; here is what that costs when we change persistence"
      ↦  §Programmers know the benefits of everything: "We do not really look at what we are getting along with that."

§1.4  [MUST][METHOD]  Do not accept tests, type systems, refactoring tools or process as a substitute for a simple design; require the design itself to be untangled enough to reason about.
      Once software reaches a certain size it is dominated by complexity regardless of testing or process, because changes require understanding and the tools do not care whether the program is simple (overrides common practice — apply as stated).
      ✗  "The checkout module is tangled, but it has full test coverage, so it is fine to keep extending it"
      ✓  "Tests are a safety net; first separate pricing from persistence in checkout so a change to one cannot require understanding the other"
      ↦  §Simplicity is a Choice: "Do not lean on your tools."

§1.5  [MUST][METHOD]  Choose the simpler construct in each of the source's pairs by default (values over state, functions over methods, polymorphism over inheritance and switching, data over syntax, set functions over loops, queues over actors and call chains, declarative data manipulation over ORM, consistency over eventual consistency), and do not choose the complex one without a stated reason.
      Complex constructs chosen without a reason accumulate unnoticed until the program can no longer be changed, while a stated reason turns each one into a deliberate tradeoff.
      ✗  Model a new feature as a mutable class hierarchy because that is the language's default style
      ✓  Model it as plain values and functions, and use mutable state only where a written requirement needs it
      ↦  §Simplicity is a Choice: "You need to not choose them, unless there is really a good reason."

§1.6  [SHOULD][METHOD]  Do not judge a design by how many components, classes or operations it has; prefer more independent parts over fewer interleaved ones, and do not split an interface down to single operations in the name of simplicity.
      Simplicity concerns interleaving, not cardinality, so counting parts pushes designs toward merging independent concerns or toward meaningless one-method interfaces, neither of which removes entanglement.
      ✗  Reject a design with four small independent components in favour of one component that does all four jobs
      ✓  Accept the four components because none depends on the internals of another
      ↦  §Simple: "It is really about the interleaving, not the cardinality."

§1.7  [SHOULD][METHOD]  When a problem is too hard to reason about, reduce the complexity of the implementation rather than expecting more effort, familiarity or skill to make it tractable.
      Tools can be installed and unfamiliar ideas learned, but mental capacity does not grow, so a problem far from the team's capabilities stays hard until its implementation is simplified.
      ✗  "The sync engine is confusing because people have not spent enough time in it yet; assign it to our strongest engineer"
      ✓  "The sync engine is confusing because conflict resolution, retry and storage are interleaved; separate them so each can be understood alone"
      ↦  §Making Things Easy: "We need to move them closer to us by making sure our implementation of them is as simple as possible."

§1.8  [SHOULD][METHOD]  Spend time simplifying a design before building on it, rather than deferring simplification in favour of a faster start.
      Simplifying is real work, but the team with genuinely simple parts makes the greatest change with the least work later, while the team that skipped it ends up pushing the elephant of existing software.
      ✗  Start building a long-lived service immediately on top of whatever structure the first prototype had
      ✓  Before extending the prototype, pull apart the parts doing more than one job, then build on the separated parts
      ↦  §Agile manifesto quote: "He is going to spend more time simplifying things up front."

§1.9  [SHOULD][METHOD]  Include an explicit entanglement check in routine team review: ask whether the last change complected anything and whether the next one will.
      Simplicity requires vigilance and does not fall out of tooling or practice, so without a recurring question entanglement is added without anyone noticing until changes become slow.
      ✗  A stand-up or review that covers only progress and blockers
      ✓  A stand-up or review that also asks "did we tie two concerns together yesterday, and does today's task tie into existing code?"
      ↦  §Simplicity is a Choice: "Try to take part of your stand-up and say: did we entangle anything?"

§2.1  [SHOULD][CODE]  Write programs that mostly manipulate immutable values and hold state only where it is needed; do not model data as stateful objects by default.
      Stateful objects interleave a value with its changes over time, so every reader must reason about when the object was modified, which makes the code harder to test and change.
      ✗  `order.add_item(item); order.recalculate_total!`
      ✓  `new_order = add_item(order, item)` returning a new value whose total is computed from its items
      ↦  §What's in your Toolkit?: "we could have written a program that mostly manipulates values, and occasionally has state."

§2.2  [SHOULD][CODE]  Use an ordinary function instead of a method that reads or mutates hidden object state.
      A function depends only on its arguments, so it is easier to test, understand, maintain and combine than a method whose behaviour depends on state not visible at the call site.
      ✗  `report.compute_totals` reading and writing `@rows` and `@totals`
      ✓  `compute_totals(rows)` returning the totals
      ↦  §What's in your Toolkit?: "An ordinary function is much, much simpler in the sense that I am talking about than a method."

§2.3  [SHOULD][CODE]  Avoid reassignable variables as much as the language allows; where state is required, keep it in one explicitly managed reference.
      A variable interleaves a name with every value it has held, so reasoning about any line requires knowing the history of assignments before it.
      ✗  `total = 0; total = apply_discount(total); total = add_tax(total)` reassigned across a long method
      ✓  `total = add_tax(apply_discount(subtotal))` with each intermediate bound once
      ↦  §What's in your Toolkit?: "Variables are things that are very complex and should be avoided as much as possible."

§2.4  [SHOULD][CODE]  Do not dispatch on the kind of a value through inheritance or an involved `switch`/`case` or pattern match when the language offers a polymorphism construct that can be extended per kind.
      Each inheritance link or kind-dispatching switch ties every kind to one place, so adding a kind means editing that place and re-reasoning about all existing kinds.
      ✗  `case shape.type when :circle ... when :square ... end` repeated in `area`, `perimeter` and `render`
      ✓  An `area` protocol or interface implemented separately for circle and square
      ↦  §What's in your Toolkit?: "Every time you write an involved switch statement, or use pattern matching, instead of a polymorphism construct, you are adding complexity to your system."

§2.5  [SHOULD][CODE]  Replace imperative index or accumulator loops with higher-level collection functions.
      An imperative loop complects a loop variable with the work being done, so the reader must track the variable's changes to know what the loop computes.
      ✗  `i = 0; result = []; while i < items.length; result << items[i].price; i += 1; end`
      ✓  `items.map(&:price)`
      ↦  §What's in your Toolkit?: "Looping we know is a form of complexity, because it is complecting a variable with the work to do."

§2.6  [SHOULD][CODE]  Describe configuration, queries and commands with ordinary data structures rather than inventing a syntax or DSL; when a DSL is chosen anyway, state the complexity it adds.
      Syntax associates meaning with order and position, so every consumer must know the grammar, whereas plain data can be read, generated and transformed with generic tools.
      ✗  A custom string mini-language `"filter age>30 sort name desc"` parsed by a hand-written parser
      ✓  `{filter: {age: {gt: 30}}, sort: {name: :desc}}`
      ↦  §Order in the Wild: "If you can use ordinary data structures to describe what you want to convey, you are much better off than introducing syntax."

§2.7  [SHOULD][CODE]  Prefer a declarative formulation over an imperative sequence of statements whenever the result can be expressed without depending on statement order.
      Reordering the statements of an imperative program breaks it, while reordering the clauses of a declarative program such as SQL does not, so declarative code survives change that imperative code does not.
      ✗  Fetch all rows, loop to filter them, then loop again to group and sum the survivors
      ✓  `SELECT region, SUM(amount) FROM sales WHERE year = 2012 GROUP BY region`
      ↦  §Order in the Wild: "Any kind of an imperative program will be trumped by a declarative program in terms of being simpler, and not having order problems."

§2.8  [CONSIDER][CODE]  Consider replacing chains of conditionals that encode business decisions with rules expressed as data.
      Conditionals interleave decision logic with control flow; the source names rules as the simpler alternative but gives no further guidance (rationale not given in source — applied conservatively).
      ✗  `if plan == :pro && seats > 10 ... elsif plan == :team && region == :eu ... end` scattered through handlers
      ✓  A table of `{plan:, min_seats:, region:, price:}` rules evaluated by one generic matcher
      ↦  §What's in your Toolkit?: "Conditionals are things we might be able to replace with rules."

§3.1  [SHOULD][CODE]  Do not represent a record of differently-meaning fields as a positional list, tuple or positional product type; use an associative record (map or hash) keyed by field name.
      When order carries meaning, every usage point depends on the position of each field, so inserting a field in the middle (name, email → name, phone, email) breaks every consumer regardless of IDE or refactoring tools (overrides common practice — apply as stated).
      ✗  `contact = ["Ann", "ann@example.com"]; email = contact[1]`
      ✓  `contact = {name: "Ann", email: "ann@example.com"}; email = contact[:email]`
      ↦  §Lists and Order: "If order matters, complexity has been introduced into the system."

§3.2  [SHOULD][CODE]  Use a set rather than a list for a collection whose order does not matter and which should contain no duplicates.
      A list forces every reader to ask whether order matters and whether duplicates are allowed, while a set states both answers in its type.
      ✗  `roles = [:admin, :editor]` checked with `roles.include?` and deduplicated by hand
      ✓  `roles = Set[:admin, :editor]`
      ↦  §Lists and Order: "So prefer that if you can."

§3.3  [SHOULD][CODE]  When a function's parameters cannot be told apart at the call site except by position, pass them as named arguments or a map; keep positional parameters only where concision is the deliberate gain and the order is self-evident.
      Positional arguments carry the same order problem as positional records, so adding or reordering a parameter means finding and fixing every call site, and swapped same-typed arguments go unnoticed.
      ✗  `create_box(10, 20, 5)` where depth, width and height are all integers
      ✓  `create_box(depth: 10, width: 20, height: 5)`
      ↦  §Order in the Wild: "But you have to know when you are choosing something like that, that you are accepting some complexity here."

§3.4  [SHOULD][ARCH]  Use a data format whose maps and lists are explicit (JSON, EDN or similar) rather than XML for exchanging data between programs.
      XML was designed for text where order matters, so a parser cannot tell whether element order is significant or whether an element may repeat, which makes XML parsers poor for data and every consumer must guess.
      ✗  `<person><name>Ann</name><email>ann@example.com</email></person>` as an API payload
      ✓  `{"name": "Ann", "email": "ann@example.com"}`
      ↦  §Order in the Wild: "Are the parsers that work with XML really good for data?"

§4.1  [SHOULD][CODE]  Represent plain information as maps or hashes rather than wrapping it in a class with information-specific accessors and methods (overrides common practice — apply as stated).
      A class wrapped around information ties logic to a representation and class names "du jour" and prevents generic data utilities from working on it; the source hedges this as true "90% of the time".
      ✗  `class PersonInfo; def get_name; @name; end; def get_address; @address; end; end`
      ✓  `person = {name: "Ann", address: "1 Main St"}`
      ↦  §Information is Simple: "You would be much better off, 90% of the time you use classes to do data things, to just use a hash instead."

§4.2  [MUST][CODE]  Do not justify wrapping information in a class on the grounds of encapsulation; reserve encapsulation for genuine implementation details.
      Information has no implementation to hide and must expose some representation through accessors anyway, so the wrapper adds verbs and coupling without encapsulating anything.
      ✗  "Wrap the address fields in an `Address` class with getters so the representation is encapsulated"
      ✓  "An address is information; pass it as a map, and encapsulate only the geocoding client that has real implementation details"
      ↦  §Encapsulation: "Encapsulation is for implementation details, and information does not have implementation."

§4.3  [SHOULD][CODE]  Write data-processing utilities (merge, select, transform, serialise) once against generic maps instead of per-class accessors.
      Per-class utilities must be rewritten for each new type and know each class's getter names, whereas generic map functions apply to all information unchanged.
      ✗  `PersonInfo#to_json`, `OrderInfo#to_json` and `InvoiceInfo#to_json` each written by hand
      ✓  One `select_keys(record, keys)` and one JSON encoder applied to every record map
      ↦  §Maps (aka hashes), Dammit!: "You want to leverage the fact that you can do generic manipulation of these things."

§5.1  [MUST][ARCH]  Make every subsystem boundary take and return plain data, with the operations (verbs) on the subsystem's interface and none on the values passed across it.
      Passing objects with methods across a boundary ties the callee to the caller's classes and language, so the subsystem cannot be moved out of process, to another thread or to another language without rewriting both sides.
      ✗  `IService.doSomethingUseful(IPersonInfo)` where `IPersonInfo` carries `getName()` and other verbs
      ✓  `IService.doSomethingUseful(person)` where `person` is a map such as `{name: "Ann"}`
      ↦  §Subsystems Must Have: "But the biggest point is: we want to take and return data."

§5.2  [SHOULD][ARCH]  Pass data between in-process components the same way it would be passed between services: as plain data structures, not object graphs.
      Teams switch to passing data only once a component moves to another server, so components built around objects must be rewritten to move, whereas components that always exchanged data move straightforwardly.
      ✗  Components exchange `ActiveRecord` objects internally, and the plan is to "convert to JSON when we split it out"
      ✓  Components exchange hashes internally, so splitting one into a service changes only the transport
      ↦  §Subsystems Must Have: "In fact, they _are_ good inside your program."

§5.3  [SHOULD][ARCH]  Test each subsystem design by asking whether it could be moved out of process, onto a different thread, or into a different language without much change; treat a no as evidence of entanglement to remove.
      A subsystem that cannot move is tied to its callers' representations or verbs, which blocks relocating, substituting or recombining it when requirements change.
      ✗  A pricing module that calls back into `Cart` and `Customer` objects it was handed and cannot run outside the Rails process
      ✓  A pricing module that takes a cart map and returns a price map, so it can run in-process today and as a service tomorrow
      ↦  §Can You Move It?: "Litmus test - can you move your subsystems?"

§5.4  [SHOULD][ARCH]  When a component fulfils more than one role, task or concept, split it into separate components before extending it.
      Design is about taking things apart, and a component holding several concerns must be dragged along whole whenever one of them is reused, substituted or moved to a different box.
      ✗  One `OrderProcessor` that validates input, charges cards, sends email and writes audit logs
      ✓  Separate validation, payment, notification and audit components composed by a thin coordinator
      ↦  §Simplicity = Opportunity: "Good design is about taking things apart."

§5.5  [SHOULD][ARCH]  Where one step's output feeds the next step, prefer handing it off through a queue over a direct call chain when the producer should stay unaffected by changes to its consumers; do not insert queues between every pair of components.
      A chain where A calls B calls C is an ordered list, so changing who consumes A's output requires editing A, whereas with a queue a new consumer can be added without touching A.
      ✗  `OrderService` calls `InventoryService`, which calls `ShippingService`, so adding analytics means editing `OrderService`
      ✓  `OrderService` puts order events on a queue; inventory, shipping and analytics each consume it independently
      ↦  §Order in the Wild: "as an architectural construct that reduces complexity, queues are really important."

§5.6  [MUST][ARCH]  Do not choose an eventually consistent data store unless a stated requirement makes a consistent one infeasible.
      Eventual consistency is very difficult to reason about, so every piece of code reading the store must account for stale and conflicting data, which multiplies the cases to understand and test.
      ✗  Choose an eventually consistent database for a new app because it is the fashionable new technology
      ✓  Choose a consistent database, and accept eventual consistency only after documenting the availability or scale requirement that rules it out
      ↦  §What's in your Toolkit?: "So do not choose it unless you really have to."

§5.7  [SHOULD][ARCH]  Do not adopt an ORM by default; evaluate it against declarative data manipulation over plain data and record why the ORM is worth its complexity.
      An ORM is one of the most complex constructs available and is chosen repeatedly because everyone does, and teams then waste an inordinate amount of time working around it.
      ✗  Start every new service with an ORM because the framework template includes one
      ✓  Query with SQL or a declarative query layer returning plain maps, and add an ORM only with a recorded reason
      ↦  §What's in your Toolkit?: "ORM is one of the most complex things you could ever touch."

§5.8  [CONSIDER][ARCH]  Consider starting with a system of data-passing components that can later be split into services, rather than designing a system of services from the first iteration.
      Splitting into services up front demands forethought about boundaries before they are understood, whereas components that already exchange data can be morphed into services when the need appears.
      ✗  Design the first iteration as six separately deployed services
      ✓  Build one deployable of components that exchange plain data, and move a component out once a reason to do so arises
      ↦  §Can You Move It?: "Why can't people morph from a system of components to a system of systems."
