package def

import (
	"time"
)

type InputGroup map[string]*Input

type OutputGroup map[string]*Output

/*
	Input specifies a data source to feed into the beginning of a computation.

	Inputs can be one of many different `Type`s of data source.
	Examples may include "tar", "git", "hadoop", "ipfs", etc.

	Inputs must specify both a `Hash` and a `URI`.
	`Input.Hash` is the content identity descriptor and will always be verified for all types of data source.
	`Input.Hash` is both identifies the data and provides integrity (in other words,
	all repeatr's input types will use a cryptographically strong hash here,
	so given a hash even an untrusted datastore is safe to use).
	Repeatr requires this to be accurate because if the inputs change, output
	consistency is impossible -- so even for plain http downloads, this is enforced.

	`Input.URI` is a secondary content lookup descriptor, like where on
	the filesystem or network information can be fetched from.
	`Input.URI` might contain extra description to answer questions like
	"which git remote url should i fetch from" or
	"which ipfs swarm do i coordinate with".

	The `URI` is *not* included in the conjecture, because repeatr understands
	that your data will be mobile -- that's why we have the `Input.Hash` take the leading role
	and why the `Input.Hash` should be sufficient to identify the information.
	Changes in the `Input.URI` field may make or break whether the data is accessible,
	but should never actually change the content of the data -- it's just supposed to talk about
	transport details; and content itself is still checked by `Input.Hash`.
*/
type Input struct {
	Type       string          `json:"type"`           // implementation name (repeatr-internal).  included in the conjecture.
	Hash       string          `json:"hash"`           // identifying hash of input data.  included in the conjecture.
	Warehouses WarehouseCoords `json:"silo,omitempty"` // secondary content lookup descriptor.  not considered part of the conjecture.
	MountPath  string          `json:"mount"`          // filepath where this input should be mounted in the execution context.  included in the conjecture.
}

/*
	Output describes where we intend to pick up data after a task completes.

	Outputs can be one of many different `Type`s of data sink.
	Examples may include "tar", "git", "hadoop", "ipfs", etc.

	`Output.MountPath` states where we should collect information from the
	task execution environment.
	After the task completes, repeatr will pick up this data, ship it off
	to storage, and also calculate a checksum of the data so we can see
	whether it matches any prior (or future) runs of this `Formula`.

	Outputs must specify a `URI`; repeatr will ship your data to this address.
	`Output.URI` has similar properties to `Input.URI` (and also similarly,
	is not included in the conjecture, because repeatr understands that
	your data can be mobile).

	The `Output.Hash` field will be filled in with a value computed
	from the data present in `Output.MountPath` after the task has completed.
	As with `Input.Hash`, the `Output.Hash` in repeatr will always be a
	cryptographically strong hash, which means it precisely describes your
	data, and makes it virtually impossible to accidentally get the same
	`Hash` as other data -- any changes to your output will always result
	in a very different `Hash` value.

	(In a content-addressable data store, repeatr may just lift the data store's
	address to use as `Output.Hash`, which is super efficient for everyone involved.
	For other more legacy-oriented systems, this may be a hash of the
	of the working filesystem right before before export.)

	Whether or not to include an `Output` in the overall `Formula`'s conjecture
	is up to you!  Many things in the world are not deterministic; repeatr
	is here to help you with the ones that should be, and stay out of the way
	for the ones that aren't.  Just set the `Output.Conjecture` boolean.

	Some examples of using `Conjecture` conditionally: if you have a job
	which does a bunch of calculations and should spit out a consistent result,
	but also does a lot of progress logging, gather those in two separate outputs.
	Mark the output of your computation in one output and set that to be
	included in the conjecture so repeatr can help you check your algorithm's
	correctness.  Now, since you may want to keep your logs for later, mark
	those as another output, and since these probably contain timestamps and
	other info that isn't *supposed* to be repeated exactly on another run,
	just set `Conjecture=false` on this one so repeatr knows not to check.

	`Output.Filters` may also be used to do a clean up pass on output files
	before committing them to storage or doing repeatr's consistency checks.
	(One typical example, which is engaged by default for you when an output
	is configured to be included in the conjecture, is setting all the file
	modification times to a standard value.)
*/
type Output struct {
	Type       string          `json:"type"`            // implementation name (repeatr-internal).  included in the conjecture (iff the whole output is).
	Hash       string          `json:"hash"`            // identifying hash of output data.  generated by the output handling implementation during data export when a task is complete.  included in the conjecture (iff the whole output is).
	Warehouses WarehouseCoords `json:"silo,omitempty"`  // where to ship the output data.  not considered part of the conjecture.
	MountPath  string          `json:"mount,omitempty"` // filepath where this output will be yanked from the job when it reaches completion.  included in the conjecture (iff the whole output is).
	Filters    *Filters        `json:"filters,omitempty"`
	Conjecture bool            `json:"cnj,omitempty"` // whether or not this output is expected to contain the same result, every time, when given the same set of `Input` items.
}

/*
	Filters are transformations that can be applied to data, either to
	normalize it for storage or to apply attributes to it before feeding
	the data into an action's inputs.

	The following filters are available:

		- uid   -- the posix user ownership number
		- gid   -- the posix group ownership number
		- mtime -- the posix file modification timestamp

	'uid', 'gid', and 'mtime' are all filtered by default on formula outputs --
	most use cases do not need these attributes, and they are a source of nondeterminism.
	If you want to keep them, you may specify	`uid keep`, `gid keep`, `mtime keep`,
	etc; if you want the filters to flatten to different values than the defaults,
	you may specify `uid 12000`, etc.
	(Note that the default mtime filter flattens the time to Jan 1, 2010 --
	*not* epoch.  Some contemporary software has been known to regard zero/epoch
	timestamps as errors or empty values, so we've choosen a different value in
	the interest of practicality.)

	Filters on inputs will be applied after the data is fetched and before your
	job starts.
	Filters on outputs will be applied after your job process exits, but before
	the output hash is computed and the data committed to any warehouses for storage.

	Note that these filters are built-ins (and there are no extensions possible).
	If you need more complex data transformations, incorporate it into your job
	itself!  These filters are built-in because they cover the most common sources
	of nondeterminism, and because they are efficient to implement as special
	cases in the IO engines (and in some cases, e.g. ownership filters, are also
	necessary for security properties an dusing repeatr IO with minimal host
	system priviledges).
*/
type Filters struct {
	UidMode   FilterMode
	Uid       int
	GidMode   FilterMode
	Gid       int
	MtimeMode FilterMode
	Mtime     time.Time
}

type FilterMode int

const (
	FilterUninitialized FilterMode = iota
	FilterUse
	FilterKeep
	FilterHost
)

var (
	FilterDefaultUid   = 1000
	FilterDefaultGid   = 1000
	FilterDefaultMtime = time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC)
)
