# godocs-client

Go HTTP client for the [godocs](https://github.com/drummonds/godocs) API.

## Usage

```go
import godocsclient "github.com/drummonds/godocs-client"

client := godocsclient.NewClient("http://localhost:8080")

// Upload a file
result, err := client.Upload("/path/to/file.pdf", "destination/path")

// Upload bytes
result, err := client.UploadBytes(data, "file.pdf", "destination/path")

// Tags
tagID, err := client.EnsureTag("my-tag")
err = client.AddTag(result.ULID, tagID)

// Metadata
err = client.UpdateMetadata(result.ULID, godocsclient.MetadataUpdate{
    Author: &author,
})
```

## Links

| | |
|---|---|
| Documentation | https://h3-godocs-client.statichost.page/ |
| Source (Codeberg) | https://codeberg.org/hum3/godocs-client |
| Mirror (GitHub) | https://github.com/drummonds/godocs-client |
| Docs repo | https://codeberg.org/hum3/godocs-client-docs |
