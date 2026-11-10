---
format: 3
---
Source: Functional Core, Imperative Shell — Gary Bernhardt (Destroy All Software screencast DAS-0072)
Scope:  Application code in any language that can express immutable values, demonstrated in a single-process Ruby terminal Twitter client; covers splitting a program into an immutable functional core and a thin imperative shell, thread hand-off of immutable values to a single I/O-owning main thread, isolated double-free testing of the core, and the explore-then-extract workflow that moves shell code into the tested core
Limitations: Evidence is one small interactive program with a roughly 160-line shell, so the rules do not say how to split a large or distributed system into several cores and shells; the source does not address error handling or failure recovery in the shell, performance or memory cost of constructing new immutable objects, persistence design beyond "write it to the database", or concurrency models other than a background thread feeding a queue; it admits converting an existing program is "not going to be easy" and gives no migration plan; its decision to leave the shell untested rests on the author's own usage and a low conditional count, with no threshold for when the shell becomes complex enough to need tests

§1.1  [SHOULD][ARCH]  When evaluating or designing a program, locate every place mutation happens and confine it to a few small pieces kept separate from the data and the core behaviour; do not require the program to match a single-core, single-shell layout before judging it.
      Mutation spread through the data and logic means every component must be reasoned about with its history and tested through stateful setup or doubles, while localised mutation leaves the bulk of the code checkable by input and output; demanding a full core/shell rewrite instead blocks improvements the source considers the more important goal.
      ✗  `Timeline`, `Cursor` and `TweetRenderer` each mutate their own fields and also write to the screen or database when called; or rejecting a change because the program is not yet one functional core inside one shell
      ✓  `Timeline`, `Cursor` and `TweetRenderer` are immutable and do no I/O, and the only reassignment and device access live in one shell file
      ↦  "and try to localize it as much as possible."

§1.2  [SHOULD][ARCH]  Put all of the program's logic and data manipulation in the functional core so that it is the bulk of the program; keep the shell limited to reading input, talking to the network and database, writing output, and updating references to core values.
      Logic left in the shell is logic that is not tested in isolation (the shell is where untested, exploratory code lives), so every decision that stays there is a decision whose regressions are only caught by using the program.
      ✗  The key handler in the shell computes the new selection index, bounds-checks it against the tweet array and chooses which tweet is visible
      ✓  The key handler calls `@cursor.down`, and `Cursor` owns index arithmetic, bounds and visible-slice logic
      ↦  "the functional core, which is the bulk of the program, containing all of the logic and the data manipulation."

§1.3  [SHOULD][CODE]  Implement "update" operations on core objects by constructing and returning a new object that combines the new data with the old, never by changing the receiver's state.
      A receiver that changes in place cannot be shared safely across threads and its tests must build it, mutate it, then inspect it, instead of asserting one input against one output.
      ✗  `def add(new_tweets); @tweets = new_tweets + @tweets; self; end`
      ✓  `def add(new_tweets); Timeline.new(new_tweets + @tweets); end`
      ↦  "I mean that it constructs a new timeline with the new tweets plus the old tweets."

§1.4  [SHOULD][CODE]  Have core code that drives an output device return the data to be output (for example an array of lines) and leave the destructive write to the shell.
      A renderer that draws directly cannot be verified by comparing returned values, so its many cases need a fake screen or a stub, and its output cannot be produced without the device present.
      ✗  `TweetRenderer#render` calls `screen.draw(line)` for each tweet it formats
      ✓  `lines = TweetRenderer.new(tweets, selected, height).render` returns lines, and the shell then passes them to the screen
      ↦  "It takes in an array of tweets, a selected tweet, and a height of the screen. And it ultimately ends up returning an array of lines of text that the screen class should draw."

§1.5  [SHOULD][CODE]  Do not flag rebinding of a local variable inside a core function as a breach of immutability; judge immutability only by whether the object's state or any other object's state changes as seen from outside the call. (overrides common practice — apply as stated)
      Treating internal local reassignment as mutation pushes core code into contortions without gaining anything, because callers and other threads cannot observe a local rebinding.
      ✗  Review rejects `TweetRenderer#render` because it assigns `lines` and later rebinds `lines = lines.first(height)`
      ✓  Review accepts the local rebinding of `lines` and rejects only a write to an instance variable or to an argument
      ↦  "The only mutation that's ever done in any of these is to rebind local variables."

§2.1  [SHOULD][ARCH]  Encapsulate the imperative shell in as few places as possible — ideally a single file — that owns standard input, standard output, the network, the database and the references to the current core values.
      When I/O and reference updates are scattered across many classes, no single place shows how the world's state reaches the program's state, and every scattered site is another untested spot that can drift out of sync.
      ✗  `Cursor` reads keystrokes, `Timeline` fetches from the API and `TweetRenderer` writes to the database, each in its own class
      ✓  One shell file reads stdin, receives timelines from the network thread, writes the database and reassigns `@cursor` and the world
      ↦  "that's all done by the imperative shell, which is encapsulated in a single file — mostly at least"

§2.2  [SHOULD][CODE]  In the shell, respond to an event by calling a core method that constructs a new value and assigning the result to the shell's reference; do not compute the new state inline in the handler.
      A handler that computes state itself puts logic in the untested shell and grows the conditional count that makes leaving the shell untested unsafe.
      ✗  `when "j" then @selected_index += 1 if @selected_index < @tweets.length - 1`
      ✓  `when "j" then @cursor = @cursor.down`
      ↦  "in both cases, we're just calling a method on the cursor that constructs a new one, and then assigning it to this instance variable."

§2.3  [SHOULD][ARCH]  Let only one thread touch standard input, standard output and the database; have background threads produce immutable values and hand them to that thread through a queue.
      When several threads touch the same device or database they can race on it and need explicit locking, whereas a single owning thread makes every such access synchronised for free.
      ✗  The timeline-stream thread fetches tweets and writes them to the database and redraws the screen itself
      ✓  The timeline-stream thread pushes a new `Timeline` into a queue; the main thread's `select` loop pulls it out, writes it to the database and updates the world
      ↦  "the main thread is the only one who touches the database, so I have no fears about multiple threads racing on the database, or racing on standard in or on standard out."

§2.4  [SHOULD][ARCH]  Share only immutable objects between threads.
      A mutable object visible to two threads can be changed by one while the other reads it, producing race conditions that need locking to prevent; an immutable one cannot be mutated in the wrong way.
      ✗  The background thread appends to the same `@timeline.tweets` array that the main thread renders from
      ✓  The background thread builds a new immutable `Timeline` and enqueues it; the main thread reads that object
      ↦  "I have no fear of a race condition or of mutating it in the wrong way because the object is immutable — so there's just no danger there."

§3.1  [SHOULD][METHOD]  Test the functional core in isolation with explicit input and output: construct real values, call the function, and assert on the returned value in a single line per case, using no mocks or stubs.
      Tests of mutating code need setup, a destructive call and a state inspection per case, and tests that use doubles force the reader to track down other classes and reason about whether the doubles match them.
      ✗  `timeline = Timeline.new([]); timeline.push(tweet); expect(timeline.tweets).to eq([tweet])`; or a `TweetRenderer` spec that stubs the screen
      ✓  `expect(Timeline.new([]).add([tweet]).tweets).to eq([tweet])`; `expect(cursor.tweets_starting_at(tweet2)).to eq([tweet2])`
      ↦  "There's no need to track down other classes because it's all data in, data out, and there's no need to reason about mocking or stubbing because there just isn't any."

§3.2  [SHOULD][CODE]  In core tests, pass real value objects rather than stubs standing in for them.
      A stub in place of a cheap value object adds a test double that can diverge from the real class without making the test any more isolated than the real value would.
      ✗  `tweet = stub(id: 1, username: "bob", text: "hi")` passed to `Cursor.new`
      ✓  `tweet = Tweet.new(1, "bob", "hi")` passed to `Cursor.new`
      ↦  "for some reason I stubbed instead of using the tweet value object ... I could easily replace those with an actual tweet value object."

§3.3  [SHOULD][METHOD]  When shell code gains conditionals whose branches are not all exercised by ordinary use of the program, extract that logic into the tested core or add tests for it; do not leave branching logic in an untested shell.
      The case for an untested shell rests on its being flat, so every added branch is a path that can break silently with nothing to catch it.
      ✗  The shell grows a multi-branch block that chooses retry, cache or reconnect behaviour on network errors and stays untested
      ✓  The retry decision moves into a core function with unit tests, and the shell only calls it and acts on the result
      ↦  "If this became more complex, then I would become afraid and I would really want to test this."

§3.4  [CONSIDER][METHOD]  Accept an untested imperative shell when it consists of flat methods with very few conditionals, and those conditionals are either required for the program to work at all or exercised constantly in normal use. (overrides common practice — apply as stated)
      Unit-testing a flat shell would require doubles for the network, terminal and database, recreating the mocking the core design removes, while the few branches it has are already proven each time the program runs.
      ✗  Requiring mocked `STDIN`, Twitter client and database tests for a shell whose only branches are the key dispatch and the event-type check in the `select` loop
      ✓  Leaving that flat shell untested and keeping its line count and conditional count low
      ↦  "There are very few conditionals in this code."

§4.1  [CONSIDER][METHOD]  Use the imperative shell as the place to explore an unfamiliar library or undecided behaviour, then, once the design is known, move that code into unit-tested core classes, typically by test-driving them and reshaping the logic to be functional.
      Test-driving code whose interface and behaviour are still unknown wastes tests on designs that will be thrown away, while leaving the explored code in the shell indefinitely leaves logic untested.
      ✗  Writing a full spec suite around a first guess at wrapping an unfamiliar Twitter library; or leaving the working exploratory drawing code in the shell permanently
      ✓  Hacking the drawing code in the shell until its behaviour is settled, then test-driving `TweetRenderer` and removing the shell version
      ↦  "once I know what it's going to do (or I at least have a decent design), then I will move the source code into unit-tested files, often by doing TDD."

§4.2  [CONSIDER][METHOD]  Extract shell code into the core in two steps: first add the new tested class and its spec without wiring it in, then in a separate change replace the untested shell code with calls to it.
      Combining the new class and the swap in one change mixes new behaviour with deleted behaviour, so a regression cannot be attributed to the new class versus the wiring, and the new class cannot be built against the still-working original.
      ✗  One commit that adds `TweetRenderer`, its spec, and deletes the shell's drawing code
      ✓  Commit "add the TweetRenderer class" touching only the new spec and production file, then commit "replace untested TweetDrawing with TweetRenderer"
      ↦  §The Refactor Cadence
