# Dockerfile References: https://docs.docker.com/engine/reference/builder/

# Start from the latest golang base image
FROM golang:1.18.0

# Add Maintainer Info
LABEL maintainer="Nadir Hamid <matrix.nad@gmail.com>"

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

ADD .gitconfig /root/.gitconfig
#RUN echo "Host bitbucket.org\n\tStrictHostKeyChecking no\n" >> /root/.ssh/config
#RUN git config --global url.ssh://git@bitbucket.org/.insteadOf https://bitbucket.org/
# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .
COPY netdiscover /bin/

RUN ls -a
# Build the Go app
RUN go build -o main .

# Expose port 80 to the outside world
EXPOSE 80

# Command to run the executable
ENTRYPOINT ["/app/entrypoint.sh"]

