package port

// CSS selectors and URL paths for the LMS.
// All selectors confirmed from live DOM inspection and network traffic analysis.
// Centralized here so the service layer can reference them without importing infrastructure.
const (
	// Login form fields (confirmed from LMS HTML)
	SelUsernameInput = "#username"
	SelPasswordInput = "input[name='password']"
	SelSubmitButton  = "input[type='submit']"

	// Login result indicators (confirmed from network traffic + DOM)
	// On success: browser redirects to /admin/ which contains .wrapper
	// On failure: browser stays at / with .alert-danger
	SelSuccessIndicator = ".wrapper"
	SelErrorIndicator   = ".alert-danger"

	// Alternative success indicators (for URL-based detection, not used in Race)
	SelDashboardHeader  = ".main-header"
	SelDashboardSidebar = ".main-sidebar"

	// Post-login data extraction
	SelUserNPM  = ".user-panel .info p" // NPM in sidebar
	SelUserName = ".user-header p"      // full name in user dropdown

	// KHS list page (main.php?op=mahasiswa_khs&act=cetak)
	SelKHSTable = ".table-bordered"

	// KHS detail page (main.php?op=mahasiswa_khs&act=cetak_detail)
	SelKHSCetakBtn = "a[href*='khs_pdf.php']"

	// KRS page (main.php?op=master_mahasiswa&act=konversi_upd_mhs)
	SelKRSSemesterInput = "input[name='semester']" // semester number input

	// KRS URL paths
	KRSDownloadPath = "/admin/cetak/krs_pdf.php"
	KRSPagePath     = "/admin/main.php?op=master_mahasiswa&act=konversi_upd_mhs"

	// KHS URL paths
	KHSListPath   = "/admin/main.php?op=mahasiswa_khs&act=cetak"
	KHSDetailPath = "/admin/main.php?op=mahasiswa_khs&act=cetak_detail"

	// Student Profile URL path
	StudentProfilePath = "/admin/main.php?op=data_mahasiswa&act=viewupdate"
)
