package storage

import (
	"context"
	"mime/multipart"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

type Client struct {
	cld *cloudinary.Cloudinary
}

func NewClient(cloudURL string) (*Client, error) {
	cld, err := cloudinary.NewFromURL(cloudURL)
	if err != nil {
		return nil, err
	}
	return &Client{cld: cld}, nil
}

type UploadResult struct {
	URL      string
	PublicID string
}

func (c *Client) Upload(file multipart.File, folder string) (*UploadResult, error) {
	ctx := context.Background()
	resp, err := c.cld.Upload.Upload(ctx, file, uploader.UploadParams{
		Folder: folder,
	})
	if err != nil {
		return nil, err
	}
	url := resp.SecureURL
	if url == "" {
		url = resp.URL
	}
	return &UploadResult{URL: url, PublicID: resp.PublicID}, nil
}

func (c *Client) Delete(publicID string) error {
	ctx := context.Background()
	_, err := c.cld.Upload.Destroy(ctx, uploader.DestroyParams{PublicID: publicID})
	return err
}
