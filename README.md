# ktop (Kubernetes Top)

Top for Kubernetes container metrics in mostly realtime. `ktop` will connect
to the Kubernetes cluster set in your Kubernetes config `KUBECONFIG`, it connect
to the Kubernetes metrics client and polls for new changes every few seconds.

`ktop` starts an interactive terminal that will show the current container `CPU`
and `Memory` metrics. You can order by CPU or Memory usage and filter based on
the namespace, pod or container name.


## Installing

Install the usual Go way:

    $ go get -u github.com/vishen/ktop

## Running

    $ ktop

## Bindings

### Key Binding

* Any ascii character entered will be used to filter
* 1 - Order by CPU usuage descending
* 2 - Order by CPU usage ascending
* 3 - Order by Memory usage descending
* 4 - Order by Memory usage ascending
* ESC - Quits the application

### Mouse Binding

* Left click - follow that particular container
* Right click - stop following the container


## TODO

```
- Order changes when ordering by CPU or MEM because we have to sort the containers each time
- Remove all magic +1, +5 numbers
- Fix hack for resizing; currently we just reduce pod and container name to reasonable size
- Remove locks from display
- Scrolling for when lines is bigger than terminal
    - Not always a problem as you can search for containers
- Configurable kubeconfig in cmdargs
- Change watch time will in interactive mode
- Show current cluster name in application heading
- Add the same for nodes?
- highlight any recent changes?
```
