# go-ordered-json

I got nerd-sniped by dax (https://twitter.com/thdxr/status/1777372160467583066) into seeing if I
could use Go to read a json file and then edit and write it back with the keys in the same order but
with a modification

pretty easy in principle, just had to write a JSON parser that parsed to a Btree instead of a Map

this is NOT a feature complete thing it's something I wrote in an hour because I was bored at work.
it's broken in 9 million ways I'm sure, and I barely know what I'm doing in Go
