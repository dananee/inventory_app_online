# Set a default commit message in case you forget to add one
m ?= "Auto-commit updates"
b ?= "main"

.PHONY: push

push:
	git add .
	git commit -m "$(m)"
	git push origin $(b)