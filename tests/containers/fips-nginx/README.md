# fips-nginx Container

This is used by the `fips.enable.https` test to verify that using
TLS works in FIPS mode by having Ignition fetch a remote resource
over HTTPS with FIPS compatible algorithms.

To build the container using command:
`./build.sh <IP>`

To run the container image using command:
`podman run -d --name fips-nginx -p 8443:8443 fips-nginx`
