PHONY: build

# This pins the versions of everything. Change if you need an upgrade.
LEAN_VERSION = 4.19.0
GO_VERSION = 1.24.3
REPL_TAG = v4.19.0

build:
	@echo "Building the docker image of the server..."
	sudo docker buildx build -f Dockerfile.provider --build-arg LEAN_VERSION=$(LEAN_VERSION) --build-arg GO_VERSION=$(GO_VERSION) --build-arg REPL_TAG=$(REPL_TAG) -t lean-provider .
	sudo docker buildx build -f Dockerfile.app --build-arg LEAN_VERSION=$(LEAN_VERSION) --build-arg GO_VERSION=$(GO_VERSION) --build-arg REPL_TAG=$(REPL_TAG) -t lean-repl .

run: clean
	@echo "Running a LEAN provider server..."
	sudo docker run -d --name lean-provider lean-provider
	@echo "Running a LEAN REPL server..."
	sudo docker run -it --rm -p 8080:8080 --volumes-from lean-provider --name lean-repl lean-repl

clean:
	@echo "Cleaning up previous LEAN provider containers..."
	@bash -c "sudo docker container stop lean-provider 2&>1 /dev/null || true"
	@bash -c "sudo docker container rm -f lean-provider 2&>1 /dev/null || true"
	@bash -c "sudo docker container stop lean-repl 2&>1 /dev/null || true"
	@bash -c "sudo docker container rm -f lean-repl 2&>1 /dev/null || true"
