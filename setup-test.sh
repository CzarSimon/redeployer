# Setup
docker rm -f httplogger
docker tag czarsimon/httplogger:0.4 czarsimon/httplogger:0.4-temp
docker run -d --name httplogger czarsimon/httplogger:0.4-temp
docker ps
echo ""

