---
format: 3
---
Source: A Philosophy of Software Design — John Ousterhout (Yaknyam Press, first edition v1.01, 2018)
Scope:  Design of methods, functions, classes, modules, subsystems and network services in any language, with examples mostly in Java and C++ plus C, Tcl and Go; object-oriented and procedural code; covers complexity as the governing design criterion, deep modules and information hiding, general- versus special-purpose interfaces, layering, error and special-case design, comments and naming, consistency and obviousness, modifying existing code, development method (strategic programming, design-it-twice, comments-first, unit tests and TDD), and designing for performance
Limitations: The book is an opinion piece built from the author's experience and student projects, with no empirical measurements (it says so of its investment-curve figure), so the 10–20% investment figure and similar numbers are suggestions; it gives no numeric threshold for "deep" or "shallow", so depth is always a comparative judgment between designs; it does not cover distributed-system architecture (service decomposition, data consistency, deployment), concurrency design beyond a note on immutable context variables, security, API versioning or backward compatibility, testing strategy beyond unit tests and failing-test-first bug fixes, or team and organisational process; it disagrees with Go's short-name style and leaves the outcome to reader complaints; and it reflects only the first edition, not later revisions

§1.1  [MUST][METHOD]  Evaluate every design decision — including whether to apply any other rule in this set — by its effect on overall system complexity (change amplification, cognitive load, unknown unknowns); when applying a rule would make the specific system harder to understand or modify, do not apply it.
      The source presents its principles as heuristics with stated limits in its "Taking it too far" sections; applying one mechanically, such as absorbing everything into one class or masking every error, produces the complexity the rules exist to prevent.
      ✗  Merge the whole application into one class because "pull complexity downward" says modules should absorb complexity
      ✓  Pull complexity into a class only when it is related to the class's functionality, simplifies its callers and simplifies its interface, and leave it out otherwise
      ↦  "The overall goal is to reduce complexity; this is more important than any particular principle or idea you read here."

§1.2  [MUST][METHOD]  When a reviewer or reader reports that code is not obvious, treat it as not obvious and change the code or its comments; do not argue that it is clear.
      Complexity is what readers experience, not writers; dismissing a reader's confusion leaves the obscurity in place for every later maintainer, who will misread the code and introduce bugs.
      ✗  Review reply: "It's clear if you read the `RaftClient` constructor — no change needed"
      ✓  Review reply: rename the entity or add the comment that answers the reviewer's question, then ask whether it is now clear
      ↦  "if a reader thinks it’s not obvious, then it’s not obvious"

§1.3  [SHOULD][ARCH]  Replace implicit dependencies with explicit, discoverable ones: when changing one value or decision requires changing others, make the relationship visible in one place that the compiler or a text search can find.
      An unknown unknown — a required change nobody can discover — can only be found by reading all the code, so in practice it surfaces as a bug after the change ships.
      ✗  Some pages hard-code a darker shade of the banner colour for emphasis, so changing `bannerBg` silently leaves mismatched emphasis colours
      ✓  The emphasis colour is derived from `bannerBg` in the same central place, so one edit updates both
      ↦  "Of the three manifestations of complexity, unknown unknowns are the worst."

§1.4  [SHOULD][CODE]  Do not judge an implementation simpler because it has fewer lines; prefer the longer version when it reduces what a reader must know to use or change it.
      Line count ignores cognitive load; a terse construct whose meaning must be reverse-engineered costs every reader time and invites misuse.
      ✗  Adopt a framework in which each feature is three lines of annotations whose behaviour cannot be determined without reading the framework's source
      ✓  Write the somewhat longer explicit code whose behaviour is evident at the call site
      ↦  "Sometimes an approach that requires more lines of code is actually simpler, because it reduces cognitive load."

§2.1  [MUST][METHOD]  Do not introduce a kludge, special case or shortcut to finish the current task faster; treat working code as necessary but not sufficient.
      Complexity accumulates from many small compromises that each look harmless; once enough have built up, every change touches several of them and the code base becomes too costly to clean up.
      ✗  Add `if (customerId == 4711)` to make tomorrow's demo pass
      ✓  Change the rule so that customer's case is handled by the general logic, even if the task takes longer
      ↦  "It’s not acceptable to introduce unnecessary complexities in order to finish your current task faster."

§2.2  [SHOULD][METHOD]  When modifying existing code, restructure it so that afterwards it looks as if it had been designed with the change in mind, rather than making the smallest diff that works; when a deadline or cross-team impact makes that impractical, choose the cleanest alternative feasible within the constraint and schedule the remainder explicitly.
      Each minimal change adds a special case or dependency, so a system maintained by smallest-possible diffs degrades with every modification until changes become slow and risky.
      ✗  Fix a bug by adding a flag parameter threaded through four methods because it is the smallest change
      ✓  Fix the bug by revising the abstraction the flag would have compensated for, so no flag is needed
      ↦  "If you’re not making the design better, you are probably making it worse."

§2.3  [SHOULD][METHOD]  Spend design effort as continual small investments, about 10–20% of development time, throughout development rather than as a large up-front design phase or as cleanup deferred until after the current crunch.
      Up-front design cannot foresee problems that appear only during implementation, and deferred cleanup is displaced by each new crunch, so design problems grow until they are too large to fix.
      ✗  Spend two months on a complete design document and freeze it; or postpone all refactoring until "after launch"
      ✓  In every iteration, fix the design problems that iteration exposed before moving on
      ↦  "the best approach is to make lots of small investments on a continual basis"

§2.4  [SHOULD][METHOD]  Make abstractions, not features, the unit of incremental development: defer an abstraction until a feature needs it, then design it completely and somewhat general-purpose at once, instead of growing it one test or feature at a time (overrides common practice — apply as stated).
      Building an abstraction piecemeal, as test-driven development and minimal-special-purpose-first agile practice encourage, leaves no point at which design happens, so the pieces never fit together and complexity accumulates.
      ✗  Write one failing test, add just enough code to pass it, repeat, and let the class's interface emerge from the sequence of tests
      ✓  When the first feature needs a text class, design its core interface (`insert`, `delete`, `changePosition`) in one sitting, then implement and test it
      ↦  "the increments of development should be abstractions, not features."

§2.5  [SHOULD][METHOD]  Before fixing a bug, write a unit test that fails because of the bug, then fix it and confirm the test passes.
      A test written after the fix may never trigger the bug, so it cannot show the fix worked and the bug can return unnoticed.
      ✗  Patch the range bug in `Buffer::copy`, then add a test that happens to use only in-range offsets
      ✓  Add a test calling `Buffer::copy` with a range extending past the end, watch it fail, then fix the code
      ↦  "Before fixing a bug, write a unit test that fails because of the bug."

§2.6  [SHOULD][METHOD]  Before committing to a module's interface or a major decomposition, sketch at least two radically different alternatives, list their pros and cons — chiefly ease of use for the calling code — and choose one or combine them; do this even when one design seems obviously right.
      The first idea is rarely the best for a hard problem, and committing to it without comparison locks in an interface that callers will work around for the life of the system.
      ✗  Implement the first idea that comes to mind, a line-oriented text API with `getLine` and `putLine`
      ✓  Sketch line-, character- and range-oriented APIs, note that the first two force callers to split lines or loop over characters, and choose the range-oriented one
      ↦  "Try to pick approaches that are radically different from each other; you’ll learn more that way."

§3.1  [MUST][CODE]  Do not add a class, method, parameter or interface unless it hides more complexity than it adds; a method whose interface and documentation are no simpler than its body is a net cost.
      Every design element is something developers must learn and a signature others depend on; a shallow one adds that cost while hiding nothing, so the system becomes harder to understand with no compensating benefit.
      ✗  `private void addNullValueForAttribute(String attribute) { data.put(attribute, null); }`
      ✓  Callers write `data.put(attribute, null)` directly, or the operation is folded into a method that does substantial work behind a simple signature
      ↦  "In order for an element to provide a net gain against complexity, it must eliminate some complexity that would be present in the absence of the design element."

§3.2  [SHOULD][CODE]  Do not split a method or class for its length alone, which is rarely a good reason; keep a long method intact when it has a simple signature and reads easily, and split only when that makes the overall system simpler (overrides common practice — apply as stated).
      Size-driven splitting multiplies shallow methods and classes whose interfaces add up to system-level complexity, and it separates related code so readers must flip between pieces to understand it.
      ✗  Break a 120-line method made of five tightly interacting sequential blocks into five private methods to satisfy a "no method over 20 lines" rule
      ✓  Keep the 120-line method with a comment before each block, because its signature is simple and the blocks must be read together
      ↦  "length by itself is rarely a good reason for splitting up a method."

§3.3  [SHOULD][CODE]  Design each interface so that its most common use needs no knowledge of rarely used features: make the common behaviour the default and put opt-outs behind a separate constructor or method.
      When the common case requires callers to assemble or enable features explicitly, every caller must learn them, and callers who forget get silently wrong behaviour such as unbuffered, slow I/O.
      ✗  `new ObjectInputStream(new BufferedInputStream(new FileInputStream(fileName)))`
      ✓  `new FileInputStream(fileName)` buffers by default, with a separate method to disable or replace buffering for the rare caller who needs it
      ↦  "interfaces should be designed to make the common case as simple as possible"

§3.4  [SHOULD][CODE]  Supply a value automatically whenever the module can determine it from its own state or from arguments the caller already passes, instead of requiring the caller to provide it.
      A caller asked for a value it cannot know either guesses wrong or must learn the module's internals, which leaks that knowledge into every call site.
      ✗  `new HttpResponse(request, "HTTP/1.1", status)`, where the version must match the one in `request`
      ✓  `new HttpResponse(request, status)` takes the protocol version from `request` and fills in the `Date` header itself
      ↦  "Whenever possible, classes should “do the right thing” without being explicitly asked."

§3.5  [SHOULD][CODE]  Do not return references to a module's internal data structures; expose operations that return the specific values callers need.
      Returning the internal collection makes its representation part of the interface, so any change to it, often made for performance, breaks every caller, and callers can corrupt internal state by mutating it.
      ✗  `public Map<String, String> getParams() { return this.params; }`
      ✓  `public String getParameter(String name)` and `public int getIntParameter(String name)`
      ↦  "it’s important to avoid exposing internal data structures as much as possible."

§3.6  [SHOULD][CODE]  Do not give a class a getter and setter for each instance variable; making a field private and then exposing it through `getFoo` and `setFoo` does not hide it (overrides common practice — apply as stated).
      Getters and setters are shallow methods that expose a variable's existence and meaning as fully as a public field, so the class gains interface clutter without information hiding and representation changes still ripple to callers.
      ✗  A `Connection` class with `getSocket()`, `setSocket()`, `getRetryCount()` and `setRetryCount()` for its private fields
      ✓  A `Connection` class exposing `send(Message)` and `close()`, with socket and retry state kept internal
      ↦  "It’s better to avoid getters and setters (or any exposure of implementation data) as much as possible."

§4.1  [MUST][ARCH]  Do not hide information that callers need to use a module correctly — durability semantics, configuration whose right value differs by use, failures they must react to — even though exposing it enlarges the interface.
      A module that omits important details is a false abstraction: callers that cannot see, for example, that messages were lost cannot build correct behaviour on top of it.
      ✗  A network module catches every network error, discards it, and continues as if the message had been delivered
      ✓  The network module retries what it can and reports lost messages and failed peers to its callers
      ↦  "If the information is needed outside the module, then you must not hide it."

§4.2  [SHOULD][ARCH]  Give each design decision — a file format, protocol, encoding or policy — a single owning module; when two modules embed the same knowledge, merge them or extract the knowledge into one module whose interface is simpler than the knowledge itself.
      Knowledge reflected in several modules forces all of them to change together when the decision changes, and back-door leakage that appears in no interface is not even discoverable until the change breaks something.
      ✗  A `ConfigReader` class and a `ConfigWriter` class each parse or emit the same file format independently
      ✓  One `ConfigFile` class owns the format and is used by both the load and save paths
      ↦  "Information leakage occurs when a design decision is reflected in multiple modules."

§4.3  [SHOULD][ARCH]  Decompose modules around the knowledge each task needs, not the order in which operations run; do not create separate read, process and write modules that each need the same format knowledge.
      Temporal decomposition puts the same knowledge in each phase's module, which duplicates code, leaks the format across modules, and makes callers invoke several methods in a fixed order.
      ✗  `RequestReader` reads an HTTP request into a string and `RequestParser` parses it, although reading requires parsing the headers to find `Content-Length`
      ✓  A single `HttpRequest` class reads and parses the request behind one method
      ↦  "When designing modules, focus on the knowledge that’s needed to perform each task, not the order in which tasks occur."

§4.4  [SHOULD][CODE]  Do not pass a variable through intermediate methods that do not use it; store such system-wide state in a context object whose reference is held by the major objects and passed only to constructors, not in a global variable.
      Pass-through variables force every intermediate signature to change when a new one is needed, and globals prevent running two instances of the system in one process, which tests often require.
      ✗  `main` passes `cert` to `m1(cert)`, which passes it to `m2(cert)`, which passes it to `m3(cert)`, its only user
      ✓  `main` stores `cert` in a `Context`; the object containing `m3` holds the `Context` reference it received in its constructor
      ↦  "The solution I use most often is to introduce a context object"

§4.5  [SHOULD][CODE]  Keep the variables in a context object immutable and limited to genuinely system-wide state; do not let the context become a grab-bag of loosely related data.
      Context variables carry most of the drawbacks of globals — nonobvious dependencies and thread-safety hazards — and mutable or miscellaneous entries make it impossible to tell why a variable exists or who changes it.
      ✗  Request handlers write per-request scratch values into the shared `Context` for later code to read
      ✓  The `Context` holds configuration and shared subsystem references fixed at startup, and per-request data travels in the request object
      ↦  "the best way to avoid problems is for variables in a context to be immutable."

§4.6  [CONSIDER][CODE]  Within a class, minimise the number of methods that touch each instance variable, and design each private method to encapsulate one piece of information or capability.
      Every method that reads or writes a field directly depends on that field's representation, so a representation change must be checked in all of them.
      ✗  Six methods each read and update `cacheEntries` and `cacheBytes` directly
      ✓  Only `lookupCached()` and `storeCached()` touch the cache fields, and other methods call them
      ↦  "try to minimize the number of places where each instance variable is used."

§5.1  [SHOULD][CODE]  Make a new module's interface somewhat general-purpose: implement only what current needs require, but define the interface in terms of the module's own basic concepts rather than the particular features of today's caller.
      An interface shaped around one caller's features gains a shallow special-purpose method for every new feature and leaks that caller's concepts into the module, so the two must be changed together.
      ✗  A text class with `backspace(Cursor cursor)`, `delete(Cursor cursor)` and `deleteSelection(Selection selection)`
      ✓  A text class with `insert(Position position, String newText)`, `delete(Position start, Position end)` and `changePosition(Position position, int numChars)`, from which the UI builds backspace and delete
      ↦  "the module’s functionality should reflect your current needs, but its interface should not."

§5.2  [SHOULD][CODE]  Do not generalise an interface so far that current callers must write substantial extra code, such as loops, to use it, and do not reduce the number of methods by loading the remaining ones with extra arguments.
      An interface too primitive for its actual use pushes repetitive, error-prone glue code into every caller and is often slow, defeating the depth the generalisation was meant to provide.
      ✗  A text class offering only single-character `insert(char c)` and `delete()`, so the editor loops to insert or delete a range
      ✓  A text class offering operations on arbitrary strings and ranges of positions
      ↦  "If you have to write a lot of additional code to use a class for your current purpose, that’s a red flag that the interface doesn’t provide the right functionality."

§5.3  [SHOULD][ARCH]  Keep a general-purpose mechanism in its own module with no knowledge of its particular uses, and put the code specialised for each use in the module that owns that use, normally a higher layer.
      Special-purpose code inside a general mechanism leaks the use case into the mechanism, so every new use requires changing the mechanism and the mechanism can no longer be understood or reused on its own.
      ✗  The text class keeps the undo list and calls back into the UI to undo selection and cursor changes
      ✓  A `History` class manages `History.Action` objects and fences; the text class supplies `UndoableInsert`, and the UI supplies `UndoableSelection` and decides where fences go
      ↦  "Special-purpose code associated with a general-purpose mechanism should normally go in a different module"

§6.1  [SHOULD][CODE]  Do not write pass-through methods that only forward their arguments to a method with a similar signature; expose the lower class directly, redistribute responsibilities, or merge the classes. Dispatchers that choose among methods, and multiple implementations of one interface, are not pass-through methods.
      A pass-through method adds interface without functionality and couples two signatures, so a change to the lower method forces a matching change above it, and it signals muddled responsibility between the classes.
      ✗  `public int getCursorOffset() { return textArea.getCursorOffset(); }` in `TextDocument`
      ✓  Callers use `TextArea` for cursor queries, or the two classes are merged so the method exists once
      ↦  "the interface to a piece of functionality should be in the same class that implements the functionality."

§6.2  [SHOULD][CODE]  Before adding a decorator or wrapper class, add the feature to the underlying class if most users want it or it is closely related, merge it into the use case or an existing decorator, or build it as a standalone class.
      Decorators duplicate the wrapped API with pass-through methods for a small amount of new behaviour, and one decorator per feature produces an explosion of shallow classes callers must assemble correctly.
      ✗  A new `ScrollableWindow` class wraps every `Window` method to add scrollbars
      ✓  Scrollbars are standalone components attached to a `Window` without re-exposing its API
      ↦  "Sometimes decorators make sense, but there is usually a better alternative."

§6.3  [SHOULD][CODE]  Make a class's interface present a different abstraction from its internal representation; do not mirror the storage structure in its methods.
      An interface that mirrors the representation leaves the conversion work to callers, where it is duplicated and scattered, and makes the class shallow.
      ✗  Text is stored as lines and the API is `getLine(int)` and `putLine(int, String)`, so the UI splits and joins lines itself
      ✓  Text is stored as lines but the API is `insert(Position, String)` and `delete(Position, Position)`, and the class splits and joins lines internally
      ↦  "the representations used internally should be different from the abstractions that appear in the interface."

§7.1  [SHOULD][CODE]  When complexity is unavoidable and related to a module's functionality, handle it inside the module rather than exposing it to callers, provided this simplifies the module's interface and the code that uses it.
      A module has more users than developers, so complexity pushed upward is paid by every caller while complexity pulled down is paid once; pulling unrelated knowledge down, however, only leaks information into the module.
      ✗  A function returns a pointer to memory it allocated and requires every caller to remember to free it
      ✓  The module that allocates the memory also frees it, so callers have nothing to remember
      ↦  "it is more important for a module to have a simple interface than a simple implementation."

§7.2  [SHOULD][CODE]  Do not export a configuration parameter the module could determine itself; before adding one, confirm that callers or administrators can choose a better value than the module can compute, and give any parameter you do add a computed default.
      Every exported parameter must be learned and tuned by every user in every installation, hand-chosen values go stale as conditions change, and a module that leaves policy to configuration solves its problem only partially.
      ✗  A transport exposes `retryIntervalMs` for administrators to tune
      ✓  The transport measures the response times of successful requests and sets the retry interval to a multiple of them
      ↦  "Thus, you should avoid configuration parameters as much as possible."

§8.1  [SHOULD][ARCH]  Combine two pieces of code only when they share information, are used together in both directions, fall under one simple higher-level concept, or cannot be understood without each other; otherwise keep them separate.
      Merging unrelated code creates a module that changes for unrelated reasons and an awkward interface callers must decode, while separating related code spreads a dependency across components developers may not know to look at.
      ✗  A hash table built into the block cache because the cache uses one, though hash tables are used without caches; one object storing the selection and cursor with booleans for which end is the cursor and whether the selection exists
      ✓  A separate hash-table module used by the block cache; the cursor as one `Position` and the selection as two `Position` values
      ↦  "This form of relationship is only compelling if it is bidirectional."

§8.2  [SHOULD][CODE]  Do not separate code into methods or classes that can only be understood by reading each other; keep such code together, for example by logging at the point where an error is detected.
      Conjoined pieces force readers to flip between locations and add interfaces that hide nothing, and a reader unaware of the dependency changes one piece without the other.
      ✗  Each `catch` block calls a single-use `NetworkErrorLogger.logRpcOpenError(req, dest, e)` defined elsewhere in the file
      ✓  The `catch` block calls `logger.log(Level.WARNING, ...)` with the message written inline
      ↦  "It should be possible to understand each method independently."

§8.3  [SHOULD][CODE]  Split a method only by extracting a subtask that can be read without the parent and used without reading its implementation, or by dividing it into methods that each have a simpler interface and most callers need only one of; do not split so that callers must invoke the pieces in sequence, passing state between them.
      A split that makes callers sequence calls and shuttle state replaces one deep method with several shallow ones and puts an ordering dependency into every caller.
      ✗  Callers must call `parseHeaders(state)` and then `parseBody(state)` to receive one request
      ✓  `readRequest()` calls a private, general-purpose `readLine()` subtask, and callers make one call
      ↦  "If the caller has to invoke each of the separate methods, passing state back and forth between them, then splitting is not a good idea."

§8.4  [SHOULD][CODE]  Eliminate repeated nontrivial code by factoring it into a method with a simple signature or by restructuring so it runs in one place; do not factor out one- or two-line snippets or snippets that would need many reference parameters.
      Repeated code means every fix must be applied to every copy and a missed copy leaves inconsistent behaviour, while a helper with a trivial body or complex signature adds an interface without reducing that burden.
      ✗  The same `LOG` statement for a too-short packet appears in each `case` of a packet-type switch
      ✓  Each `case` sets the required length and a single `LOG` statement after the switch reports short packets of every type
      ↦  "that’s a red flag that you haven’t found the right abstractions."

§9.1  [SHOULD][CODE]  Define errors out of existence: specify an operation's semantics so that inputs that would otherwise be errors have a natural, normal result (overrides common practice — apply as stated).
      Every exception is interface that each caller must handle, handler code rarely runs and so is often broken, and error-ful APIs push developers to write guard code they forget, producing runtime failures.
      ✗  `unset` throws if the variable does not exist; `substring(beginIndex, endIndex)` throws `IndexOutOfBoundsException` when an index is out of range
      ✓  `unset` ensures the variable no longer exists; `substring` returns the characters whose indexes fall in the range, possibly none
      ↦  "The best way to eliminate exception handling complexity is to define your APIs so that there are no exceptions to handle: define errors out of existence."

§9.2  [SHOULD][CODE]  Do not throw an exception merely because the module has not decided how to handle a situation, and do not reject inputs merely because they look suspicious.
      If the module's author cannot decide what to do, its callers usually cannot either, so the exception multiplies the problem across every call site and propagates through intermediate interfaces.
      ✗  `cache.remove(key)` throws `KeyNotFoundException` when the key is absent, so cleanup code wraps each call in a `catch` that ignores it
      ✓  `cache.remove(key)` guarantees the key is absent afterwards and returns normally either way
      ↦  "Generating an exception in a situation like this just passes the problem to someone else and adds to the system’s complexity."

§9.3  [SHOULD][ARCH]  When a low-level module can recover from a failure, mask it there — by retrying, resending or waiting — instead of reporting it to every caller.
      Reporting a recoverable failure upward duplicates the same recovery logic at every call site in every application, and callers that cannot recover abort, cascading the failure.
      ✗  Every application wraps each file read in its own retry loop for file-server outages
      ✓  The file-system client retries until the server responds, so applications contain no recovery code
      ↦  "it’s easier to perform the retry in one place in the NFS layer, rather than at every file system call in every application"

§9.4  [SHOULD][CODE]  Let errors that abort a request propagate to one handler near the top of the request loop, with the human-readable message generated where the error is thrown; keep these request-aborting exceptions distinct from errors fatal to the whole system.
      Catching the same class of error at each call site duplicates handler code that rarely runs and so is rarely correct, whereas one top-level handler is written once and new error sources plug into it unchanged.
      ✗  Each URL handler wraps every `getParameter` call in its own `try`/`catch` that builds an error response
      ✓  `getParameter` throws with the message "parameter 'quantity' not present in URL", and the dispatcher's single handler turns any such exception into an error response
      ↦  "rather than writing distinct handlers for many individual exceptions, handle them all in one place with a single handler."

§9.5  [SHOULD][CODE]  For errors that are rare and that this application cannot meaningfully recover from — memory exhaustion, internal data-structure inconsistency, and for most programs I/O errors on open files — print diagnostic information and abort, typically through a wrapper such as `ckalloc`; where recovery is the product's purpose, such as I/O errors in replicated storage, recover instead (overrides common practice — apply as stated).
      Handling an unrecoverable error at every site adds code that rarely runs and usually fails, and a forgotten check crashes later in a way that hides the real cause.
      ✗  Every `malloc` call is followed by its own `NULL` check and ad hoc recovery path, or by none at all
      ✓  All allocation goes through `ckalloc`, which aborts with an error message when memory is exhausted
      ↦  "The simplest thing to do in response to these errors is to print diagnostic information and then abort the application."

§9.6  [SHOULD][CODE]  Design special cases out of existence by representing the special state as an ordinary value that the normal-case code already handles.
      Each special case adds conditionals scattered through the code, and each conditional is a place where a check can be forgotten and a bug introduced.
      ✗  A `selectionExists` flag checked before every copy, delete or render of the selection
      ✓  An empty selection whose start equals its end, which copy and delete handle with no special check
      ↦  "The best way to do this is by designing the normal case in a way that automatically handles the special cases without any extra code."

§9.7  [CONSIDER][ARCH]  When a system must already have a heavyweight recovery mechanism, handle rare smaller errors by promoting them into that mechanism instead of building a separate recovery path for each; do not promote frequent errors.
      Fewer recovery mechanisms mean less code, and the shared mechanism runs more often so its bugs are found; promoting a frequent error would make recovery cost prohibitive.
      ✗  A storage server has separate code to restore one corrupted object from a backup copy, alongside its server-crash recovery
      ✓  On detecting a corrupted object the server crashes, and the existing crash recovery restores its data
      ↦  "error promotion may not make sense for errors that happen frequently."

§10.1  [MUST][CODE]  Give every method an interface comment that states its behaviour as perceived by callers, each argument and its constraints, the return value including special cases, any side effects, any exceptions and any preconditions, without describing how it is implemented (overrides common practice — apply as stated).
      A signature cannot say whether a range is inclusive or what happens when `start > end`, so without this comment callers must read the body, the method provides no abstraction, and its unstated requirements cause bugs.
      ✗  `/** Copies bytes. */ uint32_t Buffer::copy(uint32_t offset, uint32_t length, void* dest)`
      ✓  A comment stating that `copy` copies bytes starting at `offset`, that `dest` must have room for `length` bytes, and that it returns the number actually copied, fewer than `length` if the range passes the end of the buffer and 0 if there is no overlap
      ↦  "If the method has any side effects, these must be documented in the interface comment."

§10.2  [MUST][CODE]  Document each design decision exactly once, in the place a developer is most likely to look; do not repeat a called method's behaviour at its call sites, and reference external documentation such as a protocol specification or user manual instead of copying it.
      Duplicated documentation is hard to find and update in full, and copies that were missed go stale with no sign to the reader that they are wrong.
      ✗  A comment before `index.lookup(key)` explaining how `lookup` searches its tree
      ✓  No comment at the call site; `lookup`'s own interface comment documents its behaviour, and an HTTP class links to the protocol specification
      ↦  "Don’t redocument one module’s design decisions in another module."

§10.3  [SHOULD][CODE]  Give every class an interface comment describing the abstraction it provides, what each instance represents and its limitations, and comment every class variable; do not rely on "self-documenting" code for this information (overrides common practice — apply as stated).
      Rationale, usage conditions and the informal interface cannot be expressed in code, so without these comments every future developer must reconstruct them from the implementation or guess, and wrong guesses produce bugs.
      ✗  `public class Http {...}` with no comment because the code is considered self-explanatory
      ✓  A comment stating that the class lets an application receive HTTP requests and return responses, that each instance corresponds to one socket, and that it processes one request at a time
      ↦  "Every class should have an interface comment, every class variable should have a comment, and every method should have an interface comment."

§10.4  [SHOULD][CODE]  Do not write comments that repeat the adjacent code or merely rearrange the words of the entity's name; each comment must add information a reader could not get from the declaration or statement beside it.
      A comment someone could write just by looking at the adjacent code makes nothing easier to understand, takes reading time, and teaches developers that comments are worthless.
      ✗  `/* The horizontal padding of each line in the text. */` above `textHorizontalPadding`
      ✓  `/* The amount of blank space to leave on the left and right sides of each line of text, in pixels. */` above `textHorizontalPadding`
      ↦  "could someone who has never seen the code write the comment just by looking at the code next to the comment?"

§10.5  [SHOULD][CODE]  Comment each field and important variable with the precision its declaration lacks — units, whether bounds are inclusive, what null means, who frees or closes a resource, invariants — and describe what the variable represents rather than how code manipulates it.
      Vague declaration comments force readers to scan every use to learn units or meaning, and misreading them, such as pixels for characters or inclusive for exclusive, produces bugs.
      ✗  `// Current offset in resp Buffer` above `uint32_t offset;`
      ✓  `// Position in this buffer of the first object that hasn't been returned to the client.` above `uint32_t offset;`
      ↦  "When documenting a variable, think nouns, not verbs."

§10.6  [SHOULD][CODE]  Keep implementation details out of interface comments and place them inside the method body instead.
      Users reading interface documentation must wade through details they do not need, and an interface comment that has to describe the implementation shows that the method or class is shallow.
      ✗  An `IndexLookup` class comment naming the `LookupIndexKeys` and `IndexedRead` RPCs and the private limits on concurrent reads
      ✓  An `IndexLookup` class comment stating that each instance represents a single range query whose objects the caller retrieves with `getNext()`
      ↦  "It’s important to separate these two kinds of comments, so that users of an interface are not exposed to implementation details."

§10.7  [SHOULD][CODE]  Write implementation comments that state what a block of code is trying to do and why, at a higher level than the code — before each major block, before complex loops, and beside non-obvious bug-fix code citing the bug report — not comments that narrate how each statement works.
      Statement-level comments repeat the code without revealing intent, so readers cannot judge whether the code does what it should, and unexplained bug-fix code gets removed, re-creating the bug.
      ✗  `// If there is a LOADING readRpc using the same session as PKHash pointed to by assignPos, and the last PKHash in that readRPC is smaller than current assigning PKHash, then we put assigning PKHash into that readRPC.`
      ✓  `// Try to append the current key hash onto an existing RPC to the desired server that hasn't been sent yet.`
      ↦  "The main goal of implementation comments is to help readers understand what the code is doing (not how it does it)."

§10.8  [SHOULD][CODE]  When a design decision forces coordinated changes across modules, document the full set of required changes at the one place a developer must visit to make the change, such as the declaration being extended, or in a central design-notes file that short comments at each affected site refer to.
      Cross-module decisions cause many bugs because nothing in any single module reveals the other places that must change, so a developer completes a partial change and the system breaks.
      ✗  Adding a value to the `Status` enum silently requires edits to message tables, exception classes and Java bindings that nothing mentions
      ✓  A comment at the end of the `Status` enum lists the seven other places that must be updated when a value is added
      ↦  §13.7 Cross-module design decisions

§10.9  [SHOULD][CODE]  Put information future developers will need, such as the subtle problem that motivated a change, in a code comment and not only in the commit message.
      Developers rarely search the commit log, so a fix explained only there is likely to be undone by someone who unknowingly re-creates the bug.
      ✗  The commit message explains that the extra lock prevents a race during shutdown; the code has no comment
      ✓  A comment next to the lock explains the shutdown race, and the commit message may repeat it
      ↦  "This illustrates the principle of placing documentation in the place where developers are most likely to see it; the commit log is rarely that place."

§10.10  [SHOULD][CODE]  Place each comment as close as possible to the code it describes: interface comments beside the method body in the code file rather than in a separate header, and implementation comments at the narrowest scope that covers the code they describe.
      The farther a comment is from its code, the less likely a developer changing the code sees it, so distant comments go stale and readers learn to distrust all comments.
      ✗  One comment at the top of a method describing all three of its phases in detail
      ✓  A short overview of the phases at the top, then each phase's details in a comment just above that phase's first line
      ↦  "The farther a comment is from its associated code, the less likely it is that it will be updated properly."

§10.11  [SHOULD][METHOD]  Write the class interface comment and the interface comments and signatures of the main methods before writing method bodies, iterate on them until the structure is right, and write each new method's or field's comment when you declare it.
      Comments deferred until the code is done are often never written, and when they are, they repeat the code because the design reasoning has been forgotten.
      ✗  Implement and test the class, then add comments just before review
      ✓  Write the class comment and method comments with empty bodies, revise them, then fill in the bodies
      ↦  "The best time to write comments is at the beginning of the process, as you write the code."

§10.12  [SHOULD][CODE]  Treat an interface comment that cannot be both complete and short, or that must describe the major features of the implementation to be complete, as evidence of a poor abstraction, and redesign the method or variable rather than accepting the long comment.
      The interface comment is the best measure of interface complexity; a long one means callers must hold that much information to use the method correctly, and accepting it leaves a shallow module in place.
      ✗  Keep a 30-line interface comment that walks through the method's internal phases because callers need them to use it correctly
      ✓  Redesign the method so a two-sentence comment fully describes how to use it
      ↦  "if the interface comment must describe all the major features of the implementation, then the method is shallow."

§11.1  [MUST][CODE]  Use one name for each recurring concept everywhere, never use that name for anything else, and keep its meaning narrow enough that every variable with the name behaves the same; distinguish multiple instances with prefixes.
      Readers assume a familiar name has its familiar meaning, so one name covering two kinds of value produces bugs that survive review, as when a logical block number was used as a physical one and overwrote an unrelated disk block.
      ✗  `block` holds a physical disk block number in some functions and a logical file block number in others
      ✓  `diskBlock` and `fileBlock` everywhere, with `srcFileBlock` and `dstFileBlock` when one function needs two
      ↦  "never use the common name for anything other than the given purpose"

§11.2  [SHOULD][CODE]  Choose names precise enough that someone seeing a name in isolation can guess what it refers to: avoid generic words such as `count`, `status`, `x` or `result` (except for a method's actual return value), name booleans as predicates, and avoid names more specific than the entity.
      Vague names make readers deduce meaning from surrounding code, which is slow, and invite misuse when the reader's guess differs from reality.
      ✗  `getCount()`, `blinkStatus`, `VOTED_FOR_SENTINEL_VALUE`, `delete(Range selection)`
      ✓  `numIndexlets()`, `cursorVisible`, `NOT_YET_VOTED`, `delete(Range range)`
      ↦  "The most common problem with names is that they are too generic or vague"

§11.3  [SHOULD][CODE]  Use very short names such as `i` or `n` only when all uses are visible within a few lines of the declaration or the name has exactly one meaning across the system; otherwise use a descriptive name, even in languages whose style favours short names (overrides common practice — apply as stated).
      The farther a use is from its declaration, the less context a reader has, and reusing a short name such as `ch` for both character and channel creates the ambiguity that causes misuse; the source disagrees with Go's short-name culture and resolves the tension by readers — if readers find the code cryptic, the names are too short.
      ✗  `d` means data in one function and distance in another, each spanning dozens of lines
      ✓  `i` for a three-line loop index; `distance` for a value used throughout a long function
      ↦  "The greater the distance between a name’s declaration and its uses, the longer the name should be."

§11.4  [SHOULD][CODE]  When no precise, intuitive and short name fits a variable or method, treat that as a sign the entity lacks a clear purpose and refactor it, for example by splitting a variable that represents several things.
      An entity that cannot be named cleanly usually has a muddled definition, and leaving it in place means every reader must reconstruct what it actually holds.
      ✗  Settle on `stateInfo` for a variable that holds the cursor position on some paths and the selection anchor on others
      ✓  Split it into `cursorPosition` and `selectionAnchor`, each with an obvious name
      ↦  "It suggests that the variable may not have a clear definition or purpose."

§12.1  [MUST][METHOD]  Do not introduce a new convention where one already exists because it seems better; change a convention only when there is significant new information and the improvement justifies converting every existing use, and then convert them all so no trace of the old convention remains.
      Two coexisting conventions destroy the safe assumptions consistency provides, so readers must learn each situation separately and misapply the familiar pattern.
      ✗  Introduce result-type error returns in one new module of a codebase that uses exceptions everywhere else
      ✓  Use exceptions in the new module, or get agreement and migrate the whole codebase to the new style
      ↦  "Having a “better idea” is not a sufficient excuse to introduce inconsistencies."

§12.2  [SHOULD][CODE]  Before writing code in an existing file or project, find how similar things are already done there — naming case, declaration order, how a comparable design decision was made — and do the same.
      Code that deviates from local patterns forces readers to learn a second way of doing the same thing and makes them unsure which assumptions are safe.
      ✗  Add `snake_case` functions to a file in which every method is `camelCase`
      ✓  Match the file's `camelCase` names and its public-before-private ordering
      ↦  "When you see anything that looks like it might possibly be a convention, follow it."

§12.3  [SHOULD][METHOD]  Write project conventions down where developers will see them, and enforce mechanically checkable ones with a tool that blocks commits violating them.
      Conventions that live only in people's heads are broken by every newcomer, and violations that reach the repository cost review time and cause noisy diffs.
      ✗  A wiki page asks developers not to commit carriage returns in source files
      ✓  A pre-commit script rejects any modified file that contains carriage returns
      ↦  "Automated checkers work particularly well for low-level syntactic conventions."

§12.4  [SHOULD][CODE]  Do not force dissimilar things into the same name or pattern for the sake of consistency, and do not apply a design pattern to a problem it does not fit.
      Consistency helps only if "looks like an x" reliably means "is an x"; forcing a mismatch makes readers' pattern-based assumptions wrong, causing confusion and bugs.
      ✗  Wrap a one-off configuration loader in the Observer pattern because other subsystems use Observer
      ✓  Use a plain function for the loader and keep Observer for genuine publish-subscribe cases
      ↦  "don’t try to force a problem into a design pattern when a custom approach will be cleaner."

§13.1  [SHOULD][CODE]  Do not return or pass grouped values in generic containers such as `Pair`, `std::pair` or tuples; define a small named class or struct for the specific use.
      Generic element names like `getKey()` and `getValue()` hide what the values mean, so every reader must trace back to the producer; software should be designed for ease of reading, not ease of writing.
      ✗  `return new Pair<Integer, Boolean>(currentTerm, false);`
      ✓  `return new VoteResult(currentTerm, false);` with documented fields `term` and `voteGranted`
      ↦  "Thus, it’s better not to use generic containers."

§13.2  [SHOULD][CODE]  Declare a variable with the same type that is allocated for it (overrides common practice — apply as stated).
      The concrete type determines performance and thread-safety properties, so a reader who sees only the declaration may use the variable in ways the actual object does not support well.
      ✗  `private List<Message> incomingMessageList;` later assigned `new ArrayList<Message>()`
      ✓  `private ArrayList<Message> incomingMessageList;`
      ↦  "it is better to match the declaration with the allocation."

§13.3  [SHOULD][CODE]  When code behaves in a way readers will not expect — a constructor that starts threads so the program outlives `main`, an event handler invoked indirectly by a dispatcher — document the behaviour in the interface comment and, where it surprises, at the site where the reader will look.
      Readers apply conventions such as "the program exits when `main` returns", so unexpected behaviour and indirect control flow lead them to wrong conclusions about what the code does and when.
      ✗  `main` ends with `new RaftClient(myAddress, serverAddresses);` and no comment; the handler `failed()` has no comment saying who calls it
      ✓  A comment at the end of `main` says the application continues in `RaftClient`'s threads; `failed()` is documented as invoked in the dispatch thread when a transport-level error prevents an RPC from completing
      ↦  "Code is most obvious if it conforms to the conventions that readers will be expecting; if it doesn’t, then it’s important to document the behavior so readers aren’t confused."

§13.4  [SHOULD][CODE]  Before using implementation inheritance, try composition with helper classes; if inheritance is unavoidable, let the parent class's methods manage its instance variables, with subclasses only reading them or going through parent methods.
      Instance variables shared between parent and subclasses leak information across the hierarchy, so changing any class requires reading the others and, at worst, the entire hierarchy.
      ✗  Subclasses assign the parent's `protected` fields `buffer` and `length` directly
      ✓  Subclasses delegate to a shared `BufferManager` helper, or call parent methods that own `buffer` and `length`
      ↦  "Thus, implementation inheritance should be used with caution."

§14.1  [MUST][METHOD]  Do not make performance changes based on intuition: measure first to find the specific places where time is spent and to set a baseline, re-measure after the change, and back out changes that give no measurable speedup unless they also simplify the code.
      Programmers' performance intuitions are unreliable even when experienced, so unmeasured tweaks add complexity in places that do not matter while leaving the real bottleneck untouched.
      ✗  Add a cache to the parser because it "feels slow", with no profile before or after
      ✓  Profile, find that most time is spent in `Buffer::alloc`, optimise it, re-measure, and keep the change only if it is measurably faster
      ↦  "Programmers’ intuitions about performance are unreliable."

§14.2  [SHOULD][CODE]  When alternatives are equally simple, choose the one that avoids fundamentally expensive operations — network round trips, storage I/O, dynamic allocation, cache misses; when the faster design adds significant complexity or complicates interfaces, start with the simpler design unless there is clear evidence that performance will matter there.
      Ignoring performance entirely spreads small inefficiencies that make a system several times slower with no single fix, while complex optimisations made without evidence add complexity that may never pay off.
      ✗  Use an ordered map for key lookups that never need ordering; allocate an array of pointers to individually allocated structs
      ✓  Use a hash table unless ordering is needed; store the structs directly in one allocated array
      ↦  "you should always use a hash table unless you need the ordering properties provided by the map."

§14.3  [SHOULD][CODE]  When measured code is too slow, first look for a fundamental fix such as a cache or a different algorithm; only if none exists, redesign around the critical path: write the minimal common-case code, then build a clean structure close to it in which one test at the start detects all special cases and handles them off the path.
      Each special-case check and shallow layer on a hot path costs time, and scattering those checks across several thin methods makes the code both slow and hard to read.
      ✗  `Buffer::alloc` calls `Buffer::allocateAppend`, which calls `Buffer::Allocation::allocateAppend`, testing six conditions on the common path
      ✓  A single `Buffer::alloc` method tests `extraAppendBytes` once and, on the common path, enlarges the last chunk
      ↦  "Ideally, there will be a single if statement at the beginning, which detects all special cases with one test."
