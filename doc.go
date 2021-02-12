/*
Command line utility bombardier is a fast cross-platform HTTP
benchmarking tool written in Go.

Installation:
  go Get -u github.com/codesenberg/bombardier

Usage:
  bombardier [<flags>] <url>

Flags:
      --help                  Show context-sensitive help (also try --help-long
                              and --help-man).
      --version               Show application version.
  -c, --connections=125       Maximum number of concurrent connections
  -t, --timeout=2s            Socket/request timeout
  -l, --latencies             Print latency statistics
  -m, --method=GET            Request method
  -b, --body=""               Request body
  -f, --body-file=""          File to use as request body
  -s, --stream                Specify whether to stream body using chunked
                              transfer encoding or to serve it from memory
      --cert=""               Path to the Client's TLS Certificate
      --key=""                Path to the Client's TLS Certificate Private Key
  -k, --insecure              Controls whether a Client verifies the server's
                              certificate chain and host name
  -H, --Header="K: V" ...     HTTP headers to use(can be repeated)
  -n, --requests=[pos. int.]  Number of requests
  -d, --duration=10s          Duration of test
  -r, --rate=[pos. int.]      Rate limit in requests per second
      --fasthttp              Use fasthttp Client
      --http1                 Use net/http Client with forced HTTP/1.x
      --http2                 Use net/http Client with enabled HTTP/2.0
  -p, --print=<spec>          Specifies what to output. Comma-separated list of
                              values 'intro' (short: 'i'), 'progress' (short:
                              'p'), 'result' (short: 'r'). Examples:

                                * i,p,r (prints everything)
                                * intro,result (intro & result)
                                * r (result only)
                                * result (same as above)
  -q, --no-print              Don't output anything
  -o, --Format=<spec>         Which Format to use to output the result. <spec>
                              is either a name (or its shorthand) of some Format
                              understood by bombardier or a path to the
                              user-defined Template, which uses Go's
                              text/Template syntax, prefixed with 'path:' string
                              (without single quotes), i.e.
                              "path:/some/path/to/your.Template" or
                              "path:C:\some\path\to\your.Template" in case of
                              Windows. Formats understood by bombardier are:

                                * plain-text (short: pt)
                                * json (short: j)

Args:
  <url>  Target's URL

For detailed documentation on user-defined templates see
documentation for package github.com/codesenberg/bombardier/Template.
Link (GoDoc):
https://godoc.org/github.com/codesenberg/bombardier/template
*/
package main
