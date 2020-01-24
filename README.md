# Checkpoint

Checkpoint provides a simple means of recording and acting
on checkpoints in scripting environments and in particular
shell scripts. It allows the user to define appropriate
checkpoints in the execution of a script and to skip past
those points if the checkpoint has been previously 'ticked'
as done. The intent is to allow users to develop scripts
incrementally but without having to re-execute all prior
steps in a script whilst implementing/iterating on later ones.

```
source <(checkpoint use $0)
trap step EXIT

step step1 && <action>
step step2 && <action>

if step step3; then
   <action>
   checkpoint step4 && <action>
fi
```

Reaching each step implicitly 'ticks' the previous one as
complete; alternatively the current step can be explicitly marked
as complete using `step` without an argument. Note that `step` is
a shell function defined in the context of the shell via the
source statement.

## Limitations

A linear sequential control flow is currently the only supported
execution mode.

Currenly only unix shells are supported, in particular only `bash` and `zsh`
have been tested, but since little is required of the shell it should
work with all other unix-like shells. In particular, the source
step only sets an environment variable and defines the `step` shell function

## State Storage

The execution state is currently stored in the user's home directory
as files, under `$HOME/.checkpoint/...`, but other state stores
are anticipated such as dynamodb to allow for execution from other
environments such as aws lambda.