---
format: 3
---
Source: Boundaries — Gary Bernhardt (Destroy All Software talk, 33:44)
Scope:  Application code in any language that can express immutable values, with examples in Ruby across procedural, OO, functional and "FauxO" styles; covers structuring code into a functional core and an imperative shell, using values as the boundary between methods, classes, subsystems and processes, the unit-versus-integration testing strategy that follows from that split, and actor-model concurrency built on value messages
Limitations: Evidence is one 933-line interactive Twitter client plus toy examples, so the rules do not say how to split a large system into multiple core/shell units beyond hedged speculation; the source sets aside efficient data access (it admits loading every user is "not the ideal way"), error handling and failure recovery in the shell, performance cost of copying immutable values, logic programming, and concurrency models other than actors; contract and collaboration tests are mentioned but not evaluated, so these rules neither endorse nor reject them

§1.1  [SHOULD][CODE]  Give every function that makes decisions a value-in, value-out signature with no dependencies: it receives the data it decides on as parameters and returns its result, instead of fetching from a database, global or service and acting on the outcome itself.
      Code that reaches out to collaborators can only be unit-tested by replacing them with stubs and mocks, and when those doubles drift from the real classes the tests keep passing while production is broken.
      ✗  def self.sweep; User.all.select { |u| u.active? && u.paid_at < 1.month.ago }.each { |u| UserMailer.billing_problem(u) }; end
      ✓  def self.for_users(users); users.select { |u| u.active? && u.paid_at < 1.month.ago }; end
      ↦  "So we can get natural isolation by taking values and returning values and by removing all the dependencies."

§1.2  [SHOULD][CODE]  When code decides what side effect should happen, have it return a value describing that effect (the lines to draw, the users to notify) and let the caller perform it, rather than performing the effect inside the deciding code.
      A renderer or filter that writes to the screen or mailer itself cannot be checked by comparing its output, so its many cases need doubles of the stateful device and are tested against a fake instead of a value.
      ✗  `TweetListRenderer#render` calls `@screen.write_lines(...)` for each wrapped tweet
      ✓  `lines = TweetListRenderer.new(tweets, @cursor.selection, height, width).render` followed by `@screen.write_lines(0, lines)` in the view
      ↦  "It's a value representation of an action that should be taken."

§1.3  [SHOULD][CODE]  Allow methods on a value object only when each is a pure function of the object's own fields; a method that reads external state or mutates the object disqualifies it as a value.
      A value whose methods depend on outside state can no longer be built directly in a test or sent across a boundary, so the isolation and mobility the value was meant to provide are lost.
      ✗  `def slug; SlugRegistry.lookup(title); end` on `Post`
      ✓  `def slug; title.gsub(/\W+/, "-"); end` on `Post < Struct.new(:title, :body)`
      ↦  "if they're pure functions of the value of the object, I'll just call it a value."

§2.1  [MUST][ARCH]  Keep the functional core free of all I/O and long-lived mutable state — no disk, network, screen, keyboard or database access — and confine every such interaction to the imperative shell that surrounds it.
      Once a core component touches the outside world it needs integration setup or doubles to test, and its many decision paths can no longer be covered by fast isolated tests.
      ✗  `Cursor#down` reads new tweets from the Twitter API before computing the selection
      ✓  `Cursor#down` computes `Cursor.new(@tweets, @tweets.fetch(index))` from its own fields; fetching tweets happens in the shell
      ↦  "it's not allowed to mutate. So it can't touch a disk, it can't touch your screen, it can't touch your keyboard, it can't touch the network."

§2.2  [MUST][ARCH]  Keep the wrapping strict: core code calls only other core code, and only the shell calls into both the core and the stateful pieces of the system; never let a core component call back into the shell or an adapter.
      A single call from core into shell drags that shell dependency into every test of the core and turns the core's value boundary back into a call boundary that must be doubled.
      ✗  `ExpiredUsers.for_users` calls `UserMailer.billing_problem` for each late user
      ✓  `Sweeper.sweep` calls `ExpiredUsers.for_users(User.all)` and then `UserMailer.billing_problem(user)` for each result
      ↦  "The core only interacts with the core, and the shell organizes the core along with the stateful pieces of the system."

§2.3  [SHOULD][ARCH]  Put the decisions (business conditionals) in the core and the dependencies in the shell; when a shell method contains a conditional other than dispatching an external event, move that decision into a core function.
      Shell code is covered only by slow integration tests, so every decision left in it multiplies the paths those tests must exercise; the shell should be a thin layer of adapters with few paths.
      ✗  `Sweeper.sweep` evaluates `user.active? && user.paid_at < 1.month.ago` inline before mailing
      ✓  `ExpiredUsers` holds that predicate; `Sweeper` only wires `User.all`, `ExpiredUsers` and `UserMailer` together
      ↦  "The `ExpiredUsers` class contains all the decisions that are being made. ... The sweeper contains all the dependencies."

§2.4  [SHOULD][CODE]  Let the shell hold program state as a reference to the current core value, and change it only by reassigning the new value the core returns.
      If the shell computes the new state itself or the core mutates the held object, the intelligence leaks into the untested shell or the core stops being a pure value.
      ✗  `when ?j then @cursor.selection_index += 1`
      ✓  `when ?j then @cursor = @cursor.down`
      ↦  "So the core has the intelligence, but the shell has the state."

§2.5  [CONSIDER][ARCH]  For a large system, compose many small functional cores, each wrapped in its own imperative shell, that communicate by sending messages, rather than one giant core inside one shell.
      The source offers this as its expectation rather than demonstrated practice ("I think", "probably"); a single huge core/shell pair puts all coordination with the outside world into one shell.
      ✗  A 50,000-line program built as a 45,000-line functional core inside a 5,000-line imperative shell
      ✓  Several core/shell units, each owning its own I/O, exchanging values as messages
      ↦  "What probably actually happens in a well-designed system is that you have lots of functional cores surrounded by imperative shells that communicate with each other destructively."

§3.1  [SHOULD][CODE]  Inside the core, implement state-changing operations by returning a new instance built from the old one plus the change, never by mutating `self` or a field.
      Mutating methods turn the core back into stateful OO code whose results depend on call history, so it can no longer be tested by comparing returned values or passed safely across a boundary.
      ✗  `def eat(food); @stomach << food; end`
      ✓  `def eat(food); Walrus.new(@name, @stomach + [food]); end`
      ↦  "Instead, `eat` makes a new walrus that is the old walrus with the new food in its stomach."

§3.2  [SHOULD][CODE]  In core code, transform collections by building a new collection (`map`, `select`) rather than iterating for side effects (`each`); treat an `each` in the core as a sign of a destructive operation that belongs in the shell.
      An `each` is a no-op unless its body mutates something, so its presence shows that the code has side effects and is not part of the functional core.
      ✗  `walruses.each { |walrus| walrus.eat(Cheese.new) }` in a core class
      ✓  `walruses.map { |walrus| walrus.eat(Cheese.new) }` in the core; `.each { |user| UserMailer.billing_problem(user) }` only in the shell
      ↦  "If you write Ruby code and the word `each` shows up, there's something destructive."

§3.3  [SHOULD][CODE]  Do not flag chained calls on immutable values as Law of Demeter violations; a chain like `cursor.down.down.selection` expresses the same state transitions as sequential calls on a stateful object (overrides common practice — apply as stated).
      Rejecting such chains pushes reviewers toward reintroducing mutable objects or verbose temporaries for code that carries no hidden coupling, because each call returns a fresh value.
      ✗  Review comment requiring `cursor.down.down.selection.should == tweet2` to be rewritten to avoid reaching through returned objects
      ✓  Accept `cursor.down.down.selection.should == tweet2` as written
      ↦  "you might look at this and see a Law of Demeter violation, but that's because OO has poisoned your mind."

§4.1  [SHOULD][METHOD]  Test the functional core with isolated unit tests that cover its many paths, and test the imperative shell with a small number of integration tests that only check the pieces are wired together.
      The core has many paths and few dependencies, which fast isolated tests handle well; the shell has many dependencies and few paths, so testing complex cases through it yields many slow tests.
      ✗  Every expiry edge case of the billing sweep is exercised through a test that boots the app, hits the database and captures email
      ✓  Expiry edge cases are tested against `ExpiredUsers.for_users`; one integration test checks that `Sweeper.sweep` mails what it is given
      ↦  "has lots of paths but few or hopefully no dependencies, which is exactly what isolated unit testing is good at."

§4.2  [SHOULD][CODE]  In core tests, build real value objects as inputs and assert equality on the returned value, instead of stubbing inputs and setting mock expectations on collaborators.
      Tests built on doubles can call methods that do not exist on the real collaborators, so they pass while production breaks; real values make the boundary itself real.
      ✗  `bob = stub(:active? => true, :paid_at => 2.months.ago); UserMailer.should_receive(:billing_problem).with(bob); Sweeper.sweep`
      ✓  `bob = User.new(:active => true, :paid_at => 2.months.ago); ExpiredUsers.for_users([bob]).should == [bob]`
      ↦  "you're using real values in your tests, so you know that you're calling methods that actually exist"

§4.3  [SHOULD][METHOD]  When a unit test needs many doubles, or doubles nested several calls deep, treat it as a design problem and restructure the code under test toward value boundaries instead of adding more doubles.
      Piling on mocks hides the design flaw the test is exposing and widens the surface on which doubles can fall out of sync with real classes.
      ✗  A test that mocks six collaborators, two of them three method calls deep, and is merged as is
      ✓  The decision logic is extracted into a function that takes the needed values as arguments, and its test uses no doubles
      ↦  "if you're mocking out six dependencies and two of them are mocked three levels deep, three method calls deep, you have a design problem."

§4.4  [SHOULD][METHOD]  Do not adopt integration tests as the primary testing strategy for a large system; use them only for the shell's wiring.
      With N conditionals there are on the order of 2^N paths, too many to choose among, and suite runtime grows superlinearly as tests and code are added together, ending in multi-hour suites.
      ✗  Replace isolated unit tests with full-stack tests to avoid mock drift, across the whole application
      ✓  Remove the need for mocks by moving decisions into a value-based core, and keep full-stack tests for the shell
      ↦  "So integration testing does not work as a primary testing strategy for a large, well-tested system, as far as I can tell."

§4.5  [CONSIDER][METHOD]  When the shell's only conditional is dispatching external events (such as a key-press `case`), you may leave it without automated tests and verify it by running the program (overrides common practice — apply as stated).
      A shell with no decisions has only one path per event, so a manual run exercises essentially all of it; the condition fails as soon as the shell gains any other conditional, which then needs a test or a move into the core.
      ✗  Leaving a shell untested that also checks `if tweets.count > limit` before rendering
      ✓  Leaving untested a `TimelineView#key` whose only branch is the `case key` dispatch to core calls
      ↦  "The only conditional in the shell is the case statement that we saw that was choosing between keys."

§5.1  [CONSIDER][ARCH]  When introducing concurrency, prefer the actor model and pass values, not shared mutable objects, as the messages between actors, so that method, class, subsystem and process boundaries can be moved without rewriting the code on either side.
      The source presents this as its strongest belief among concurrency models rather than a requirement; values make every result a potential message, while shared mutable objects tie the two sides to one thread and one call boundary.
      ✗  Concurrent stages share and mutate one `User` record through synchronous calls
      ✓  `Sweeper` pushes each `User` value into the `ExpiredUsers` inbox, which pushes late users into the `BillingProblemNotification` inbox
      ↦  "Values in your system afford shifting process boundaries, but really this is a special case of values in your system affording shifting boundaries of any kind"
