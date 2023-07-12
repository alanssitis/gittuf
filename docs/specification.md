# gittuf Specification

Last Modified: March 21, 2023

Version: 0.1.0-draft

## Introduction

This document describes gittuf, a security layer for Git repositories. gittuf
applies several key properties part of the
[The Update Framework (TUF)](https://theupdateframework.io/) such as
delegations, secure key distribution, key revocation, trust rotation, read /
write access control, and namespaces to Git repositories. This enables owners of
the repositories to distribute (and revoke) contributor signing keys and define
policies about which contributors can make changes to some namespaces within the
repository. gittuf also protects against reference state attacks by extending
the Reference State Log design which was originally described in an
[academic paper](https://www.usenix.org/conference/usenixsecurity16/technical-sessions/presentation/torres-arias).
Finally, gittuf can be used as a foundation to build other desirable features
such as cryptographic algorithm agility, the ability to store secrets, storing
in-toto attestations pertaining to the repository, and more.

This document is scoped to describing how TUF's access control policies are
applied to Git repositories. It contains the corresponding workflows for
developers and their gittuf clients. Note that gittuf is designed in a manner
that enables other security features. These descriptions will be in standalone
specifications alongside this one, and will describe modifications or extensions
to the "default" workflows in this document.

## Definitions

### Git Reference (Ref)

A Git reference is a "simple name" that typically points to a particular Git
commit. Generally, development in Git repositories are centered in one or more
refs, and they're updated as commits are added to the ref under development. By
default, Git defines two of refs: branches (heads) and tags. Git allows for the
creation of other arbitrary refs that users can store other information as long
as they are formatted using Git's object types.

### Actors

In the context of a Git repository, an actor is any user who contributes changes
to the repository. This may be to any file tracked by the repository in any Git
ref. In gittuf, actors are identified by the signing keys they use when
contributing to the repository. A policy that grants an actor the ability to
make a certain change in fact grants it to the holder of their signing key.
Verification of any action performed in the repository depends, among other
factors, on the successful verification of the action's signature using the
expected actor's public or verification key.

### State

State describes the expected values for tracked refs of the repository. It is
identified by the tip or last entry of the
[reference state log](#reference-state-log-rsl). Note that when inspecting
changes to the state of the repository, a workflow may only consider state
updates relevant to a particular ref.

## gittuf

To begin with, gittuf carves out a namespace for itself within the repository.
All gittuf-specific metadata and information are tracked in a separate Git ref,
`refs/gittuf`.

### Reference State Log (RSL)

Note: This document presents only a summary of the academic paper and a
description of gittuf's implementation of RSL. A full read of the paper is
recommended.

The Reference State Log contains a series of entries that each describe some
change to a Git ref. Each entry contains the ref being updated, the new location
it points to, and a hash of the parent RSL entry. The entry is signed by the
actor making the change to the ref.

Given that each entry in effect points to its parent entry using its hash, an
RSL is a hash chain. gittuf's implementation of the RSL uses Git's underlying
Merkle graph. Generally, gittuf is designed to ensure the RSL is linear but a
privileged attacker may be able to cause the RSL to branch, resulting in a
forking attack.

The RSL is tracked at `refs/gittuf/reference-state-log`, and is implemented as a
distinct commit graph. Each commit corresponds to one entry in the RSL, and
standard Git signing mechanisms are employed for the actor's signature on the
RSL entry.

#### Normal RSL Entries

These entries are the standard variety described above. They contain the name of
the reference they apply to and a commit ID. As such, they have the following
structure.

```
RSL Entry

ref: <ref name>
commit: <commit ID>
```

#### RSL Annotation Entries

Apart from regular entries, the RSL can include annotations that apply to prior
RSL entries. Annotations can be used to add more information as a message about
a prior entry, or to _explicitly_ mark one or more entries as ones to be
skipped. This semantic is necessary when accidental or possibly malicious RSL
entries are recorded. Since the RSL history cannot be overwritten, an annotation
entry must be used to communicate to gittuf clients to skip the corresponding
entries. Annotations have the following schema.

```
RSL Annotation

entryID: <RSL entry ID 1>
entryID: <RSL entry ID 2>
...
skip: <true/false>
-----BEGIN MESSAGE-----
<message>
------END MESSAGE------
```

#### Example Entries

TODO: Add example entries with all commit information. Create a couple of
regular entries and annotations, paste the outputs of `git cat-file -p <ID>`
here.

### Actor Access Control Policies

Note: This section assumes some prior knowledge of the TUF specification.

There are several aspects to how defining the access privileges an actor has.
First, actors must be established in the repository unambiguously, and gittuf
uses TUF's mechanisms to associate actors with their signing keys. TUF metadata
distributes the public keys of all the actors in the repository and if a key is
compromised, new metadata is issued to revoke its trust.

Second, TUF allows for defining _namespaces_ for the repository. TUF's notion of
namespaces aligns with Git's, and TUF namespaces can be used to reason about
both Git refs and files tracked within the repository. Namespaces are combined
with TUF's _delegations_ to define sets of actors who are authorized to make
changes to some namespace. As such, the owner of the repository can use gittuf
to define actors representing other contributors to the repository, and delegate
to them only the necessary authority to make changes to different namespaces of
the repository.

Policies for gittuf access are defined using a subset of TUF roles. The owners
of the repository hold the keys used to sign the Root role that delegates trust
to the other roles. The top level Targets role and any Targets roles it
delegates to contain restrictions on protected namespaces. The specifics of the
delegation structure vary from repository to repository as each will have its
own constraints.

A typical TUF delegation connects two TUF Targets roles. Therefore, delegations
can be represented as a directed graph where each node is a Targets role, and
each edge connects the delegating role to a delegatee role for some specified
namespace. When verifying or fetching a target, the graph is traversed using the
namespaces that match the target until a Targets entry is found for it. The
Targets entry contains, among other information, the hashes and length of the
target. gittuf applies this namespaced delegations graph traversal to Git and
also incorporate RSLs and Git's implicit change tracking mechanisms.

In gittuf, the delegations graph is similarly traversed, except that it
explicitly does not expect any Targets metadata to contain a target entry.
Instead, the delegation mechanism is used to identify the set of keys authorized
to sign the target such as an RSL entry or commit being verified. Therefore, the
delegation graph is used to decide which keys git actions should trust, but no
targets entries are used.  Any key which delegated trust up to this part of the 
namespace (including the last delegation), is trusted to sign the git actions.

This mechanism is employed when verifying both RSL entries for Git ref updates
_and_ when verifying the commits introduced between two ref updates. The latter
option allows for defining policies to files and directories tracked by the
repository. It also enables repository owners to define closed sets of
developers authorized to make changes to the repository. Note that gittuf does
not by default use Git commit metadata to identify the actor who created it as
that may be trivially spoofed.

Another difference between standard TUF policies and those used by gittuf is a
more fundamental difference in expectations of the policies. Typical TUF
deployments are explicit about the artifacts they are distributing. Any artifact
not listed in TUF metadata is rejected. In gittuf, policies are written only to
express _restrictions_. As such, when verifying changes to unprotected
namespaces, gittuf must allow any key to sign for these changes. This means that
after all explicit policies (expressed as delegations) are processed, and none
apply to the namespace being verified, an implicit `allow-rule` is applied,
allowing verification to succeed.

In summary, a repository secured by gittuf stores the Root role and one or more
Targets roles. Further, it embeds the public keys used to verify the Root role's
signatures, the veracity of which are established out of band. The metadata and
the public keys are stored as Git blobs and updates to them are tracked through
a standalone Git commit graph. This is tracked at `refs/gittuf/policy`. The RSL
MUST track the state of this reference so that the policy namespace is protected
from reference state attacks. Further, RSL entries are used to identify
historical policy states that may apply to older changes.

## Example

Consider project `foo`'s Git repository maintained by Alice and Bob. Alice and
Bob are the only actors authorized to update the state of the main branch. This
is accomplished by defining a TUF delegation to Alice and Bob's keys for the
namespace corresponding to the main branch. All changes to the main branch's
state MUST have a corresponding entry in the repository's RSL signed by either
Alice or Bob.

Further, `foo` has another contributor, Clara, who does not have maintainer
privileges. This means that Clara is free to make changes to other Git branches
but only Alice or Bob may merge Clara's changes from other unprotected branches
into the main branch.

Over time, `foo` grows to incorporate several subprojects with other
contributors Dave and Ella. Alice and Bob take the decision to reorganize the
repository into a monorepo containing two projects, `bar` and `baz`. Clara and
Dave work exclusively on bar and Ella works on baz with Bob. In this situation,
Alice and Bob retain their privileges to merge changes to the main branch.
Further, they set up delegations for each subproject's path within the
repository. Clara and Dave are only authorized to work on files within `bar/*`
and Ella is restricted to `baz/*`. As Bob is a maintainer of foo, he is not
restricted to working only on `baz/*`.

## Main Repository gittuf Behavior

WIP

gittuf can be deployed at the main repository that all developers push changes
to. This instance of gittuf must behave differently from regular gittuf clients
because multiple developers may be pushing and fetching from the main repository
at any given point in time.

### What are the responsibilities of gittuf at the main repository?

The main repository uses gittuf to:
* receive an update from a developer
* perform gittuf verification on the update
* make update available to all developers if gittuf verification succeeds
* serialize update handling so that the linearity of the RSL is maintained

### How can gittuf perform verification at the repository?

gittuf can perform verification in several ways at the server. The first option
is a simple pre-receive hook that is invoked every time an update is pushed to
the server. The hook is provided a summary of the changes to the RSL and the
updated Git reference, i.e., all the information necessary for gittuf
verification. If the verification is successful, the push is allowed to succeed.

The other option is to use a CI system or have gittuf verification implemented
in the server itself.

### Where must a gittuf clients send an update to?

A gittuf client pushing an update changes _two_ Git references: the reference
updated by the developer and changes to the RSL. The client must push both
changes
[atomically](https://git-scm.com/docs/git-push#Documentation/git-push.txt---no-atomic)
so that either both references are updated or neither at the main repository.

The client, broadly speaking, has two options: either it pushes directly to the
main repository's RSL and the updated reference, or it does not, instead pushing
to another namespace at the main repository. Let us consider each.

#### Pushing directly to the RSL and updated reference

As the push is atomic, the repository does not enter an invalid state where only
the RSL or the updated reference is updated. This option works well in low
traffic scenarios, where pushes are few and far between. This is because in the
case of multiple parallel pushes, verification may be invoked for each in
parallel.  This is the case for pre-receive hooks. The first one to succeed
"wins", updating the repository state and leading to the other pushes failing.
This is not ideal as gittuf verification scales by the number of RSL entries and
files modified in an update. Larger changes will, therefore, fail more due to
concurrent smaller changes. Note that Git implements a lock on its internal
store, but this does not block pushes prior to the hook invocation (TODO: test,
this is based off existing documentation).

If the invocation of verification is not via a pre-receive hook, the repository
can reject other new pushes when verifying one. This mechanism must also ensure
the RSL state and the updated reference are not exposed to clients until after
verification succeeds, as clients must not receive an unverified RSL entry.

#### Pushing to staging namespaces in the repository

In this case, the gittuf client pushes to staging namespaces, such as
`refs/gittuf/staging/reference-state-log` and
`refs/gittuf/staging/<git-reference>`. The repository verification can be
triggered using any mechanism. As the staging area namespaces are consistently
generated by all gittuf clients, they act as an implicit lock to new pushes. A
gittuf client cannot push to the staging area when a prior push is being
verified.

If the verification succeeds, the verifying mechanism must update the actual RSL
and the updated reference. In addition, regardless of the status of
verification, the staging namespaces must be removed to allow other gittuf
clients to push to.

The staging namespace cannot be used with a pre-receive hook as it must reflect
a push in progress. The post-receive hook is a better fit if a Git server-side
hook is to be used.

## Verification Workflow

There are several aspects to verification. First, the right policy state must be
identified by walking back RSL entries to find the last change to that
namespace. Next, authorized keys must be identified to verify that commit or RSL
entry signatures are valid.

### Identifying Authorized Signers for Protected Namespaces

When verifying a commit or RSL entry, the first step is identifying the set of
keys authorized to sign a commit or RSL entry in their respective namespaces.
With commits, the relevant namespaces pertain to the files they modify, tracked
by the repository. On the other hand, RSL entries pertain to Git refs. Assume
the relevant policy state entry is `P` and the namespace being checked is `N`.
Then:

1. Validate `P`'s Root metadata using the TUF workflow, ignore expiration date
   checks.
1. Begin traversing the delegations graph rooted at the top level Targets
   metadata. Set `current` to top level Targets and `parent` to Root metadata.
1. Create empty set `K` to record keys authorized to sign for `N`.
1. While `K` is empty:
   1. Load and verify signatures of `current` using keys provided in `parent`.
      Abort if signature verification fails.
   1. Identify delegation entry that matches `N`, `D`.
   1. If `D` is the `allow-rule`:
      1. Explicitly indicate any key is authorized to sign changes as `N` is not
         protected. Returning empty `K` alone is not sufficient.
   1. Else:
      1. If repository contains metadata with the role name in `D`:
         1. Set `parent` to `current`, `current` to delegatee role.
         1. Continue to next iteration.
      1. Else:
         1. Set `K` to keys authorized in the delegations entry.
1. Return `K`.

### Verifying Changes Made

In gittuf, verifying the validity of changes is _relative_. Verification of a
new state depends on comparing it against some prior, verified state. For some
ref `X` that is currently at verified entry `S` in the RSL and its latest
available state entry is `D`:

1. Fetch all changes made to `X` between the commit recorded in `S` and that
   recorded in `D`, including the latest commit into a temporary branch.
1. Walk back from `S` until a state entry `P` is found that updated the gittuf
   policy namespace. This identifies the policy that was active for changes made
   immediately after `S`.
1. Validate `P`'s metadata using the TUF workflow, ignore expiration date
   checks.
1. Walk back from `D` until `S` and create an ordered list of all state updates
   that targeted either `X` or the gittuf policy namespace. During this process,
   all state updates that affect `X` and the policy namespace must be recorded.
   Entries pertaining to other refs MAY be ignored. Additionally, all annotation
   entries must be recorded using a dictionary where the key is the ID of the
   entry referred to and the value the annotation itself. Each entry referred to
   in an annotation, therefore, must have a corresponding entry in the
   dictionary.
1. The verification workflow has an ordered list of states
   `[I1, I2, ..., In, D]` that are to be verified.
1. For each set of consecutive states starting with `(S, I1)` to `(In, D)`:
   1. Check if an annotation exists for the second state. If it does, verify if
      the annotation indicates the state is to be skipped. It true, proceed to
      the next set of consecutive states.
   1. If second state changes gittuf policy:
      1. Validate new policy metadata using the TUF workflow and `P`'s contents
         to established authorized signers for new policy. Ignore expiration
         date checks. If verification passes, update `P` to new policy state.
   1. Verify the second state entry was signed by an authorized key as defined
      in P.
   1. Enumerate all commits between that recorded in the first state and the
      second state with the signing key used for each commit. Verify each
      commit's signature using public key recorded in `P`.
   1. Identify the net or combined set of files modified between the commits in
      the first and second states as `F`.
   1. If all commits are signed by the same key, individual commits need not be
      validated. Instead, `F` can be used directly. For each path:
         1. Find the set of keys authorized to make changes to the path in `P`.
         1. Verify key used is in authorized set. If not, terminate verification
            workflow with an error.
   1. If not, iterate over each commit. For each commit:
      1. Identify the file paths modified by the commit. For each path:
         1. Find the set of keys authorized to make changes to the path in `P`.
         1. Verify key used is in authorized set. If not, check if path is
            present in `F`, as an unauthorized change may have been corrected
            subsequently. This merely acts as a hint as path may have been also
            changed subsequently by an authorized user, meaning it is in `F`. If
            path is not in `F`, continue with verification. Else, request user
            input, indicating potential policy violation.
   1. Set trusted state for `X` to second state of current iteration.

## Actor Workflows

These workflows were originally written during the prototyping phase and need to
be updated. Note: This document expects readers to be familiar with some of
Git's default user workflows.

### Initializing a new repository -- `git init`

Alongside the standard creation of a new Git repository, gittuf also signs and
issues version 1 of the Root metadata and the top level Targets metadata. An out
of band process may be used (such as a root signing ceremony) to generate these
files, and therefore, pre-signed metadata may be passed in. The public keys used
to verify the Root role must also be included.

All of these files are stored in the `refs/gittuf/policy` namespace. The tree
object must contain two subtrees: `keys` and `metadata`. The root public keys
are stored as Git blobs and recorded in the `keys` tree object and the metadata
blobs are recorded in the `metadata` tree object.

#### Edge Case -- Running `init` on an existing repository

`git init` has no impact in an existing repository. However, there may be uses
to running `gittuf init` to (re-)initialize the TUF Root for the repository. If
a TUF Root already exists, gittuf MUST exit with a warning and allow users to
forcefully overwrite the existing TUF Root with a new one. Once again, out of
band processes may be necessary to bootstrap the Root metadata.

### Making changes -- `git add`, `git commit`, and `git merge`

gittuf applies access control policies to files tracked in the repository based
on the author of the commits modifying them. As such, no changes are necessary
to the standard commit workflows employed by developers. However, to benefit
from the gittuf's guarantees, all commits SHOULD be signed by their authors.

### Making changes available to other users -- `git push`

The RSL is updated when users are ready to push changes for some ref to a remote
version of the repository. There are some modifications to this workflow from
what is described in the RSL academic paper. First, the remote's RSL is fetched
and its entries are evaluated against the current state of the target ref. If
changes were made to the ref remotely, they need to be incorporated and the
local changes must be reapplied. Further, any updates to the policy namespace
must also be applied locally. Once this process is complete (it may take
multiple passes if the target ref receives a lot of activity on the remote),
gittuf creates a provisional entry in the local RSL.

This entry is provisional because before the remote can be updated with the new
status of the target ref and the RSL, gittuf executes the
[verification workflow](#verifying-changes-made) with the provisional entry.
This means that prior to changes being pushed, the verification workflow ensures
the commits and the entry are all valid as per the latest available policy on
the remote.

If verification passes, the target ref and the RSL entry are pushed to the
remote, _after_ checking that more entries have not been created on the remote
when verification was in progress locally. If this is the case, the provisional
entry is deleted, upstream changes are fetched, and the entire process is
repeated.

## Recovery Workflows

### Recovering from accidental changes and pushes

There are several scenarios here. If a user makes changes locally and tries to
push them to the blessed copy, it should be quite easy to detect and reject the
changes. A pre-receive hook on the server can be employed to ensure the client
is also sending valid metadata for the set of changes. If not, the operation
must be terminated.

In situations where server-side hooks cannot be used (or trusted), maintainers
of the repositories can correct the record for the affected refs and sign new
RSL entries indicating the correct locations. Clients that employ gittuf are
always secure as they will reject changes that fail validation.

TODO: evaluate if consecutive state verification fails on clients behind the
times. Should recovery rewrite non-valid RSL entries? Defeats the purpose?

### Recovering from a developer compromise

If a developer's keys are compromised and used to make changes to the
repository, maintainers must immediately sign updated policies revoking their
keys. Further, maintainers may reset the states of the affected refs and sign
new RSL entries with corrected states.

TODO: evaluate if consecutive state verification fails on clients behind the
times. Should recovery rewrite non-valid RSL entries? Defeats the purpose?
