package codegen

import (
	"fmt"
	"strings"
)

// FileOperationType describes the type of file operation detected.
type FileOperationType int

const (
	FileOpNone     FileOperationType = iota
	FileOpUpload                     // multipart/form-data or file parameter
	FileOpDownload                   // binary/octet-stream response
)

// DetectFileOperation inspects an OpenAPI operation for file upload/download patterns.
func DetectFileOperation(operation map[string]any) FileOperationType {
	opType := FileOpNone

	// Check for file upload: multipart/form-data in consumes or requestBody
	if consumes, ok := operation["consumes"].([]any); ok {
		for _, c := range consumes {
			if s, ok := c.(string); ok && s == "multipart/form-data" {
				opType = FileOpUpload
			}
		}
	}

	// Swagger 2.0: parameters with type: file
	if params, ok := operation["parameters"].([]any); ok {
		for _, rawParam := range params {
			if param, ok := rawParam.(map[string]any); ok {
				if stringValue(param["type"]) == "file" {
					opType = FileOpUpload
				}
			}
		}
	}

	// OpenAPI 3.x: requestBody with multipart/form-data
	if reqBody, ok := operation["requestBody"].(map[string]any); ok {
		if content, ok := reqBody["content"].(map[string]any); ok {
			if _, ok := content["multipart/form-data"]; ok {
				opType = FileOpUpload
			}
		}
	}

	// Check for file download: binary/octet-stream in produces or response content
	if produces, ok := operation["produces"].([]any); ok {
		for _, p := range produces {
			if s, ok := p.(string); ok && s == "application/octet-stream" {
				if opType == FileOpNone {
					opType = FileOpDownload
				}
			}
		}
	}

	// OpenAPI 3.x: response content with octet-stream
	if responses, ok := operation["responses"].(map[string]any); ok {
		for _, rawResp := range responses {
			if resp, ok := rawResp.(map[string]any); ok {
				if content, ok := resp["content"].(map[string]any); ok {
					if _, ok := content["application/octet-stream"]; ok {
						if opType == FileOpNone {
							opType = FileOpDownload
						}
					}
				}
			}
		}
	}

	return opType
}

// generateFileUploadCode returns Go code for building a multipart form-data request.
func generateFileUploadCode() string {
	return `func buildMultipartRequest(url, method string, input map[string]any) (*http.Request, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for key, val := range input {
		switch v := val.(type) {
		case string:
			if strings.HasPrefix(v, "@") {
				filePath := strings.TrimPrefix(v, "@")
				file, err := os.Open(filePath)
				if err != nil {
					return nil, fmt.Errorf("open file %s: %w", filePath, err)
				}
				defer file.Close()
				part, err := writer.CreateFormFile(key, filepath.Base(filePath))
				if err != nil {
					return nil, fmt.Errorf("create form file: %w", err)
				}
				if _, err := io.Copy(part, file); err != nil {
					return nil, fmt.Errorf("copy file data: %w", err)
				}
			} else {
				if err := writer.WriteField(key, v); err != nil {
					return nil, fmt.Errorf("write field %s: %w", key, err)
				}
			}
		default:
			data, _ := json.Marshal(v)
			if err := writer.WriteField(key, string(data)); err != nil {
				return nil, fmt.Errorf("write field %s: %w", key, err)
			}
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}
`
}

// generateFileDownloadCode returns Go code for handling binary response downloads.
func generateFileDownloadCode() string {
	return `func handleBinaryResponse(resp *http.Response) map[string]any {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return mcpError(-32603, fmt.Sprintf("failed to read binary response: %v", err))
	}

	contentDisposition := resp.Header.Get("Content-Disposition")
	filename := "download"
	if contentDisposition != "" {
		if _, params, err := mime.ParseMediaType(contentDisposition); err == nil {
			if f, ok := params["filename"]; ok {
				filename = f
			}
		}
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return mcpTextResult(fmt.Sprintf("{\"filename\":%q,\"size\":%d,\"contentType\":%q,\"data\":%q}",
		filename, len(data), resp.Header.Get("Content-Type"), encoded))
}
`
}

// fileOpsImports returns the import paths needed for file operations code.
func fileOpsImports(opType FileOperationType) []string {
	switch opType {
	case FileOpUpload:
		return []string{"bytes", "encoding/json", "io", "mime/multipart", "os", "path/filepath", "strings"}
	case FileOpDownload:
		return []string{"encoding/base64", "io", "mime"}
	default:
		return nil
	}
}

// generateFileOpsCode returns the appropriate file helper functions.
func generateFileOpsCode(opType FileOperationType) string {
	var b strings.Builder
	switch opType {
	case FileOpUpload:
		b.WriteString(generateFileUploadCode())
	case FileOpDownload:
		b.WriteString(generateFileDownloadCode())
	}
	return b.String()
}

// generateFileHandlerSnippet returns the code snippet to be used inside a handler
// for file operations.
func generateFileHandlerSnippet(opType FileOperationType, method, fullURL string) string {
	switch opType {
	case FileOpUpload:
		return fmt.Sprintf(`	req, err := buildMultipartRequest(%q, %q, input)
	if err != nil {
		return mcpError(-32603, fmt.Sprintf("failed to build multipart request: %%v", err))
	}
`, fullURL, strings.ToUpper(method))
	case FileOpDownload:
		return fmt.Sprintf(`	req, err := http.NewRequest(%q, %q, nil)
	if err != nil {
		return mcpError(-32603, fmt.Sprintf("failed to create request: %%v", err))
	}
`, strings.ToUpper(method), fullURL)
	default:
		return ""
	}
}
