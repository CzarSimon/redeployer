echo "Deploying httplogger=$1"

docker run -d --name httplogger $1