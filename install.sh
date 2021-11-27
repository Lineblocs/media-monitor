cp -rf keys/key /root/.ssh/id_rsa
chmod 700 /root/.ssh/id_rsa
echo "Host bitbucket.org\n\tStrictHostKeyChecking no\n" >> /root/.ssh/config
git config --global url.ssh://git@bitbucket.org/.insteadOf https://bitbucket.org/
# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
#go mod init lineblocs.com/smudge/node
#echo "downloading now.."
export GOPRIVATE=bitbucket.org/infinitet3ch go mod download
#go build -o main .
#make
