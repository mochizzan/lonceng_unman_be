package port

// BrowserSession represents an authenticated browser session with the LMS.
// The service layer uses this interface; infrastructure implements it.
type BrowserSession interface {
	// Navigate loads the given URL and waits for the page to be ready.
	Navigate(url string) error

	// Eval executes JavaScript on the page and returns the result as a string.
	Eval(js string) (string, error)

	// ElementAttribute returns the value of an attribute on the first element matching the selector.
	ElementAttribute(selector, attr string) (string, error)

	// ElementExists returns true if an element matching the selector exists on the page.
	ElementExists(selector string) (bool, error)

	// ElementHref returns the href attribute of the first element matching the selector.
	ElementHref(selector string) (string, error)

	// DownloadPDF downloads a PDF from the given URL and saves it to savePath.
	// Returns the filename and byte count.
	DownloadPDF(url, savePath string) (string, int, error)

	// DownloadImage downloads an image from the given URL and saves it to savePath.
	// Returns the filename, byte count, and any error.
	DownloadImage(url, savePath string) (string, int, error)

	// Close releases the browser session resources.
	Close() error
}
