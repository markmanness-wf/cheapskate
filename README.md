# Cheapskate

Demo service using frugal.

## Generating the frugal interface

Run the following:-
```
docker run -u $UID -v "$(pwd):/data" drydock.workiva.org/workiva/frugal:35054 frugal --gen=go stingy.frugal
```

