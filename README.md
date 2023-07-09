# content-addressable-store

A Go service with potential for improvement.

If you have Go and Docker installed on your machine, you can run the service as shown below:

```bash
# Start a datadog agent
export DD_API_KEY="<your-api-key>"
docker compose up -d --wait agent

# Run the application
export DD_SERVICE="content-addressable-store"
go run .
```

If that's working, you can use another shell session to upload a big file.

```bash
# Create a 250MB file
dd if=/dev/urandom of=250.mb bs=1M count=250

# Upload the file using curl
curl -X POST http://localhost:8080/store --data-binary @250.mb
```
