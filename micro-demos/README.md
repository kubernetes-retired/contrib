# Kubernetes micro-demos

This is a collection of highly-targetted demonstrations of Kubernetes features.
The demos are all text-based and automated, making them perfect for customer
meetings, meetups, or just showing off to your colleagues.

## Running the demos

To run these demos you need `pv` and `tmux` installed, and you need `kubectl`
in your PATH.

Some of the demos try to act in faster-than-real time.  For best results:
  * SSH to your kubernetes-master and set the following flags (in this order):
    * kube-controllermanager: --pod-eviction-timeout=10s

Before running a demo, make sure your cluster is demo-ready.  The `reset.sh`
script is provided for that.

## Writing new demos

Each demo lives in its own directory.  The bulk of the logic lives in
`util.sh`.

Demos should be small and focused - 2 to 3 minutes each.

Demos should be repeatable.  Make sure you are not relying on timing effects.
If you need to `sleep`, you might have a problem.

Demos should be self-contained.  If you are depending on something being done
before-hand, don't.  Do it in the demo script.

Demos should be single-terminal.  Use `tmux` to split the window to show
multiple parallel things.

