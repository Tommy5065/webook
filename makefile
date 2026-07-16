.PHONY:mockgen
mock:
	@mockgen -destination=internal/service/mock/sms/mock_sms.go -package=mock_sms  ./internal/service/sms Service
	@docker run -p 9000:9000 -p 9001:9001 -v $HOME/minio/data:/mnt/data -v $HOME/minio/minio.license --name oss-server  minio/minio:RELEASE.2025-09-07T16-13-09Z server /mnt/data --license /minio.license --console-address ":9000" --address ":9001"