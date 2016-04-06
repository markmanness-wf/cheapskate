# Cheapskate

Demo service using frugal. This service supports a single API method, ```GetQuote```,
and also implements the [Workiva Base Interface](https://github.com/Workiva/messaging-sdk/tree/master/idl/api).

You can build everything by running the ```build.sh``` script. The instructions
below are primarily useful if you want to do things manually.

## Generating the frugal interface

You first need to fetch the [Workiva Base Interface](https://github.com/Workiva/messaging-sdk/tree/master/idl/api) files.
You need both the base_model.frugal and the base_service.frugal files to be
present in the same directory as this file.

Run the following commands to generate your go code:-
```
docker run -u $UID -v "$(pwd):/data" drydock.workiva.org/workiva/frugal:35054 frugal --gen=go:package_prefix=github.com/danielrowles-wf/cheapskate/gen-go/ stingy.frugal
docker run -u $UID -v "$(pwd):/data" drydock.workiva.org/workiva/frugal:35054 frugal --gen=go:package_prefix=github.com/danielrowles-wf/cheapskate/gen-go/ base_service.frugal
docker run -u $UID -v "$(pwd):/data" drydock.workiva.org/workiva/frugal:35054 frugal --gen=go:package_prefix=github.com/danielrowles-wf/cheapskate/gen-go/ base_model.frugal
```

## Fix missing import statement in generated go code

The generated go code doesn't import the frugal libraries, due to a bug.  This
will result in errors when building the cheapskate binary, such as:
```
../gen-go/workiva_frugal_api/f_baseservice_service.go:23: undefined: frugal in frugal.FContext
```

To resolve this, manually add an import for ```github.com/Workiva/frugal/lib/go```
to the following files:-
 * gen-go/stingy/f_stingyservice_service.go
 * gen-go/workiva_frugal_api/f_baseservice_service.go
