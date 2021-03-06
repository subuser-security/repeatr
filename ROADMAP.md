Roadmap
=======

- [x] a formula should give us the language to name and distribute a permanent, consistent, reproducible process.

"Software that works should continue to work" is repeatr's raison d'être.

This dialogue should never occur again:

> Human 1: "Oh, you need to install graphics drivers on your computer.  `apt-get install jockey-gtk`"  
> Human 2: "...that doesn't work"  
> Human 1: "??!  ...oh, I guess that doesn't exist on that version of ubuntu anymore..."  
> Human 2: "Can you just give me the working copy from your computer?"  
> Human 1: "*... No.  No I can't.*"  

Things shouldn't break like this.
If Human 1 has a working system, they should be able to give Human 2 a working system.

Thus: a repeatr formula should be a thing that you can pastebin to another human.
When you do this, it should be everything they need to get a working piece of software.
And they should get exactly the same piece of software as you did.

This is our objective for the future: formulas should give us the language
to describe what a reproducible process is, in black and white.
Having absolute terms for this, which should "just work", allows us to drive
the culture of accountability further than ever before.
With formulas, we can build a (Ulysses Compact)[https://en.wikipedia.org/wiki/Ulysses_pact]
with ourselves to make the best behaviors into the defacto norm.

---

- [x] pluggable executor should provide consistent, easy, sandboxed execution
  - [x] runc (docker-style containers)
  - [x] chroot (available-everywhere backcompat)
  - [ ] virtual machines

Execution engines that perform sandboxing are an essential part of providing the firepower
to make formulas meaningful.
It's one thing to describe a process; it's another to demonstrate it *doing* things.

Sandboxing makes it possible to *show* reproducibility -- or, rapidly demonstrate
where it isn't happening, and thus make it better -- by making clean environments
a common and easy tool.
Think if it like "Mandatory Sanity Control" instead of "Discretionary Sanity Control".

Repeatr executors are pluggable.
The core behaviors of running a process should behave the same across different executors,
though some offer better isolation than others,
and different executors also vary in how wide a variety of base systems the can support
as well as their performance overheads.

---

- [x] provide a consistent interface to many different Content-Addressable Storage systems:
  - [x] git (any git transport)
  - [x] tarballs in local files
  - [x] tarballs over http
  - [x] tarballs in amazon s3
  - [x] tarballs in google cloud storage
  - [ ] ipfs
  - [ ] venti
  - [ ] ...more!

Why so many?  Because the future is hard to read, and because different strokes for different folks.

- The plain tarball stuff is really useful for exporting to other systems, and also the easiest thing by far to pack onto a floppy.
- IPFS is much more powerful, deduplicates, and has clever bit-torrent-like transfer, but also requires the most online/interactive servers.
- Git, even though it's too task-specific to be a good general CAS, is incredibly handy for software developers to import their work.

Diversity is strength here.  As long as we can pin hashes, we can paper over the other differences.

---

- [x] make `repeatr save` (saves data from localhost to a CAS warehouse) work anywhere
  - [x] uploading local files is easy
  - [ ] cross-platform support and testing
- [ ] make `repeatr load` (warehouse -> you) work anywhere (without requiring an execution formula)

Saving and loading data from any of the storage systems repeatr supports should be a breeze.
It should be as cross-platform as possible.
And it should work *without any of the container/execution engines*.

(Even if most of our common container engines are linux specific,
uploading data to a storage warehouse or fetching job results on a windows machine should Just Work.
Also, gluing together a provisioning system with repeatr that feeds into some *other* execution engine
we don't have explicit support for should be an easy job.)

---

- [ ] make `repeatr save` and `repeatr load` degrade gracefully when they have only low privileges

By default repeatr tries to commit all filesystem attributes to storage, which means restoring data
might sometimes require elevated/root privileges... but since we already have support for e.g. 
filtering uid/gid attributes, `save` and `load` commands should be able to work within those bounds.
Some situations might mandate loss of fidelity (e.g. can't preserve uid bits on windows filesystems),
but we should support that usage (albeit with notices about the potential limitations).

---

- [ ] a naming service that maps `{human-readable-name} -> {hash}` so we can smoothly and safely distribute updates
  - [ ] cryptographically secure updates (e.g. with TUF)
  - [ ] local name overrides (dev mode shouldn't require the friction of keygen)
  - [ ] audit logging (optionally) for changes over time

Pinning hashes in a formula is critical to the "software that works should continue to work" story,
but it's also important to have a good mechanism to ship updates and fixes to existing systems.

Whether or not a user wants to accept these updates, and on what schedule they wish to do so, remains entirely the user's choice.
That's why we build this *on top* of the hash/pinning system: updates should never be a "surprise!  prod broke and you can't revert!" thing.

Building this naming system on top of the formula layer also means we score nice unix-y composability:
someone can still use all the CAS data model and deterministic distribution story, while building their own update and naming system,
and that's gonna be just as supported as our own nameservice.
(As a POC, repeatr's own builds do this right now: they're just templated by a bash script.)

---

- [ ] convenient syntax for pipelineing several computations

Developing and packaging software with repeatr should be awesome.
Just like modern build tools increasingly focus on deterministic compute because it allows caching and thus going Faster,
repeatr should do the same thing between project and library boundaries.


