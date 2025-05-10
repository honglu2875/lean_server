PHONY: build

LEAN_VERSION = 4.19.0

build:
	@echo "Building the docker image of the server..."
	sudo docker buildx build -f Dockerfile.provider --build-arg LEAN_VERSION=$(LEAN_VERSION) -t lean-provider .
	sudo docker buildx build -f Dockerfile.app --build-arg LEAN_VERSION=$(LEAN_VERSION) -t lean-repl .

run: clean
	@echo "Running a LEAN provider server..."
	sudo docker run -d --name lean-provider lean-provider
	@echo "Running a LEAN REPL server..."
	sudo docker run -it --rm -p 8080:8080 --volumes-from lean-provider lean-repl

clean:
	@echo "Cleaning up previous LEAN provider containers..."
	@sudo docker container stop lean-provider 2&>1 /dev/null || true 
	@sudo docker container rm -f lean-provider 2&>1 /dev/null || true 
