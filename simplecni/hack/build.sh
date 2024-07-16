CURRENT_DIR=$(dirname $0)
pushd ${CURRENT_DIR}/../
go build .
popd

pushd ${CURRENT_DIR}/../cniplugin/
go build 
popd
sudo docker build . -f ${CURRENT_DIR}/../deploy/Dockerfile -t simplecni:v0.0.1

rm -f ${CURRENT_DIR}/../cniplugin/cniplugin
rm -f  ${CURRENT_DIR}/../simplecni
