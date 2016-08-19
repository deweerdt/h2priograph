# h2priograph

`h2priograph` is a tool that transforms the output of Chrome's HTTP2
net-internal tab into a graph, showing how H2 requests issued by
Chrome depend on each other.

# Build

In addition to having `go` installed, running `make` should be all
that's needed to build h2priograph.

# Example

- Navigate to a given HTTP/2 connection under chrome://net-internals/#http2
- Select all and copy, `ctrl-A` followed by `ctrl-C`
- Paste the contents to a file:
  On MacOSX: `pbpaster > fastly`
- Generate the dot file:
  `h2priograph -file=fastly > fastly.dot`
- Generate an image (requires `dot` from `graphviz`)
  `dot -Tpng fastly.dot > fastly.png`

 # Sample graph

![Sample graph](https://github.com/deweerdt/h2priograph/blob/master/sample/fastly.png)

