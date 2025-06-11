package utils

import (
    "context"
    "log"
    "mime/multipart"
    "os"
    "strings"
    "github.com/cloudinary/cloudinary-go/v2"
    "github.com/cloudinary/cloudinary-go/v2/api/uploader"
    "github.com/joho/godotenv"
)

type CloudinaryService struct {
    cld *cloudinary.Cloudinary
}

func NewCloudinaryService() (*CloudinaryService, error) {
    err := godotenv.Load()
    if err != nil {
        log.Println("Warning: Error loading .env file, relying on environment variables:", err)
    }

    cloudName := os.Getenv("CLOUDINARY_CLOUD_NAME")
    apiKey := os.Getenv("CLOUDINARY_API_KEY")
    apiSecret := os.Getenv("CLOUDINARY_API_SECRET")

    if cloudName == "" || apiKey == "" || apiSecret == "" {
        log.Println("Error: Cloudinary environment variables (CLOUDINARY_CLOUD_NAME, CLOUDINARY_API_KEY, CLOUDINARY_API_SECRET) are not set")
        return nil, err
    }

    cld, err := cloudinary.NewFromParams(cloudName, apiKey, apiSecret)
    if err != nil {
        log.Println("Cloudinary setup error:", err)
        return nil, err
    }
    return &CloudinaryService{cld: cld}, nil
}

func (cs *CloudinaryService) UploadImage(file multipart.File, folder string) (string, error) {
    ctx := context.Background()
    uploadResult, err := cs.cld.Upload.Upload(ctx, file, uploader.UploadParams{
        Folder: folder,
    })
    if err != nil {
        log.Printf("Upload failed: %v", err)
        return "", err
    }
    log.Printf("Successfully uploaded image to Cloudinary: %s", uploadResult.SecureURL)
    return uploadResult.SecureURL, nil
}

func (cs *CloudinaryService) DeleteImage(publicID string) error {
    ctx := context.Background()
    _, err := cs.cld.Upload.Destroy(ctx, uploader.DestroyParams{
        PublicID: publicID,
    })
    if err != nil {
        log.Printf("Failed to delete image from Cloudinary: %v", err)
        return err
    }
    log.Printf("Successfully deleted image from Cloudinary: %s", publicID)
    return nil
}

func ExtractPublicIDFromURL(url string) string {
    parts := strings.Split(url, "/")
    if len(parts) < 7 {
        log.Println("Invalid Cloudinary URL format:", url)
        return ""
    }
    publicIDWithExt := parts[len(parts)-1]
    publicID := strings.Split(publicIDWithExt, ".")[0]
    folderPath := strings.Join(parts[7:len(parts)-1], "/")
    if folderPath != "" {
        return folderPath + "/" + publicID
    }
    return publicID
}

