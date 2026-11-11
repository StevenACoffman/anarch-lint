---
format: 3
---
Source: "Worse Is Better" by Jeff Atwood (Coding Horror, 11 Aug 2004), commenting on and quoting Richard Gabriel's "The Rise of 'Worse is Better'"
Scope:  Language-agnostic software design judgement: choosing between simple and theoretically superior designs, the implementation-versus-interface simplicity tradeoff, object structure (inheritance versus plain objects and interfaces), build-versus-reuse, configuration storage, and platform/framework selection
Limitations: A three-minute opinion post, not a method: it gives no measure of simplicity, so "simpler" must be judged by counting components, abstractions and concepts; it does not say when complexity is unavoidable (safety-critical, security or data-integrity requirements where the "right thing" is mandatory) and these rules must not be used to drop a stated requirement; it quotes only fragments of Gabriel's essay and none of its follow-up, so Gabriel's full four-criterion comparison (simplicity, correctness, consistency, completeness) is not covered; it does not address licensing, security or maintenance-health checks when reusing open-source code; its platform examples (x86, the Windows registry, COM, J2EE) are from 2004 and illustrate the principle rather than ranking today's technologies; and it endorses implementation simplicity over interface simplicity without saying where that stops, which conflicts with deep-module advice elsewhere

§1.1  [MUST][ARCH]  When two candidate designs both satisfy the stated requirements, choose the simpler one by removing whatever the requirements do not need, even when the other is theoretically superior or more elegant.
      A design chosen for theoretical correctness over simplicity is harder to understand, so it is misused, breaks in ways its maintainers cannot diagnose, and loses out to simpler competitors that keep working.
      ✗  Propose a plugin registry with a dependency-injection container and lifecycle hooks for an app that has two fixed output formats, because it is "the proper architecture"
      ✓  Propose a `switch` on the two formats, and accept that it is less general, because it meets the requirement with nothing extra to understand
      ↦  "always err on the side of simplicity"

§1.2  [SHOULD][ARCH]  Do not adopt a solution whose correct use depends on every future maintainer understanding an advanced technique; choose the version a typical team member can read and modify safely.
      Assuming everyone can handle the clever solution is optimism, and when that is wrong the less expert maintainer changes code they do not understand and introduces defects.
      ✗  Implement retry logic as a hand-rolled lock-free state machine using atomic compare-and-swap so it is "maximally efficient"
      ✓  Implement retry logic as a plain loop guarded by a mutex that any reviewer can follow
      ↦  "Don't presume that everyone's smart enough to handle the fancy complex solution"

§1.3  [SHOULD][METHOD]  After a design or change works, make a removal pass: delete each element that can go without breaking a requirement, and treat the work as finished only when nothing more can be removed.
      Complexity that is never actively subtracted builds up, and every element left in place is one more thing each later reader must understand and each later change must preserve.
      ✗  Merge as soon as the tests pass, leaving an unused configuration flag, a speculative abstract base class and a helper called from one place
      ✓  Once the tests pass, remove the unused flag, collapse the single-implementation base class and inline the one-caller helper, then merge
      ↦  "keep taking complexity away ... when you cannot remove any more – you're done"

§1.4  [SHOULD][ARCH]  Do not reject a simple solution that meets the requirements just because it is inelegant or offends design taste.
      Turning down the working simple option on aesthetic grounds sends the team to the complex option, along with the comprehension and survival costs of §1.1.
      ✗  Reject a cron job that runs a shell script nightly because "a proper event-driven pipeline would be cleaner"
      ✓  Ship the cron job because it meets the nightly requirement, and note the inelegance without treating it as a defect
      ↦  "We should strive to build simple solutions whenever possible, even if we have to"

§1.5  [CONSIDER][ARCH]  Prefer incrementally improving an existing, widely used simple design over replacing it with a clean-slate design that is theoretically superior. (Source illustrates this with an example rather than stating it as a rule; severity lowered.)
      A crude design that keeps being refined builds up compatibility and hardening that a clean-slate replacement does not have, and the source's example of the superior replacement (Itanium) failed while the crude incumbent (x86) survived.
      ✗  Replace a working, widely used wire protocol with a new, theoretically cleaner protocol that every client must adopt at once
      ✓  Extend the existing protocol with a backward-compatible field and keep refining it in place
      ↦  "honed to an incredibly sharp edge"

§2.1  [SHOULD][ARCH]  When handling an edge case completely inside a routine would greatly enlarge its implementation, and the caller's alternative is a simple check-and-retry, keep the implementation simple and return an explicit incomplete-operation error instead (overrides common practice — apply as stated).
      Building the complete handling into the implementation (backing out and restoring partial state) makes the component larger and harder to get right; Gabriel's New Jersey design accepts a slightly harder interface to keep it small. The condition limits this to cases where the caller's extra work is a trivial test and loop, and this overrides the "pull complexity downward" default.
      ✗  Make a `Read()` that can be interrupted mid-transfer save and restore its internal buffers so it always appears to complete atomically
      ✓  Have `Read()` always return promptly with an `EINTR`-style error when interrupted, and document that callers retry
      ↦  "implementation simplicity was more important than interface simplicity"

§2.2  [MUST][CODE]  When calling an interface that can return an incomplete-operation or try-again error, check for that error at every call site and retry or propagate it; never treat the call as having succeeded without checking.
      A simple-implementation interface stays correct only when callers do their part, and a caller that skips the check silently loses work or acts on partial results.
      ✗  `n, _ := conn.Read(buf); process(buf)`
      ✓  `for { n, err = conn.Read(buf); if !errors.Is(err, syscall.EINTR) { break } }; if err != nil { return err }; process(buf[:n])`
      ↦  "A correct user program, then, had to check the error code to determine whether to simply try the system routine again."

§3.1  [SHOULD][CODE]  Do not introduce a class hierarchy when a single plain object or record meets the need.
      Inheritance adds a base class, overriding rules and coupling between parent and child that every reader has to trace, which is complexity a plain object does not carry.
      ✗  `class Shape {}; class Point extends Shape { x; y }` where no other `Shape` subtype exists
      ✓  `class Point { x; y }`
      ↦  "when a simple object will do"

§3.2  [SHOULD][CODE]  When several types need to be interchangeable, express the shared contract as an interface rather than a common base class.
      A base class ties every implementer to inherited state and behaviour, so a change to the parent breaks children that only needed the method signature.
      ✗  `abstract class Storage { protected cache; save() {...} }` extended by `DiskStorage` and `S3Storage`
      ✓  `interface Storage { save(data) }` implemented independently by `DiskStorage` and `S3Storage`
      ↦  "when you can use an interface"

§3.3  [SHOULD][METHOD]  Before writing new code for a general-purpose capability, look for an existing library or product that provides it, and use that unless it fails a stated requirement.
      Code written in-house is code the team must understand, test and maintain for good; an established existing solution moves that complexity off the team.
      ✗  Write a custom CSV parser with hand-rolled quote and escape handling
      ✓  Use the language's standard or a well-established CSV library
      ↦  "Why even write code at all when you can buy or steal open-source it?"

§4.1  [SHOULD][ARCH]  Store application configuration in simple plaintext files owned by each application rather than in a single shared binary configuration store.
      A complex shared binary store can be corrupted by losing a few bytes, which wipes every application's configuration at once, whereas plaintext files can be read, diffed and repaired by hand and fail one application at a time.
      ✗  Keep all services' settings in one binary database blob that a custom tool must decode
      ✓  Give each service its own `.ini`, `.config` or YAML file in version control
      ↦  "Lose a few bytes and it's corrupt; wave bye-bye to all your registry data."

§4.2  [CONSIDER][ARCH]  Prefer a plain web service or simple library over a heavyweight component-object or "enterprise" framework when both can meet the requirement. (Source states this as a rhetorical preference; severity lowered.)
      Heavyweight frameworks bring their own registration, lifecycle and deployment machinery that the team must learn and debug, on top of the business problem itself.
      ✗  Expose two internal operations through COM components or J2EE Enterprise JavaBeans with container-managed lifecycles
      ✓  Expose the two operations as a small HTTP endpoint
      ↦  "Wouldn't you rather just build a web service?"
