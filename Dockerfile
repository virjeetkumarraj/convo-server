FROM golang:1.12

# Add Maintainer Info
LABEL maintainer="Nathanael Gavin <ngavinsir@gmail.com>"

# Set the Current Working Directory inside the container
WORKDIR $GOPATH/src/github.com/ngavinsir/convo-server

# Copy everything from the current directory to the PWD(Present Working Directory) inside the container
COPY . .

# Download all the dependencies
# https://stackoverflow.com/questions/28031603/what-do-three-dots-mean-in-go-command-line-invocations
RUN go get -d -v ./...

# Install the package
RUN go install -v ./...

# This container exposes port 8080 to the outside world
EXPOSE 30000

# Run the executable
CMD ["convo-server"]