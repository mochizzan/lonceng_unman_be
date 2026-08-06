package browser

import "lonceng_unman_be/internal/domain/port"

// Re-export constants from domain/port so infrastructure code that
// imports this package continues to compile.
const (
	// Login form fields
	SelUsernameInput = port.SelUsernameInput
	SelPasswordInput = port.SelPasswordInput
	SelSubmitButton  = port.SelSubmitButton

	// Login result indicators
	SelSuccessIndicator = port.SelSuccessIndicator
	SelErrorIndicator   = port.SelErrorIndicator

	// Alternative success indicators
	SelDashboardHeader  = port.SelDashboardHeader
	SelDashboardSidebar = port.SelDashboardSidebar

	// Post-login data extraction
	SelUserNPM  = port.SelUserNPM
	SelUserName = port.SelUserName

	// KHS selectors and paths
	SelKHSTable    = port.SelKHSTable
	SelKHSCetakBtn = port.SelKHSCetakBtn

	// KRS selectors and paths
	SelKRSSemesterInput = port.SelKRSSemesterInput

	// KRS URL paths
	KRSDownloadPath = port.KRSDownloadPath
	KRSPagePath     = port.KRSPagePath

	// KHS URL paths
	KHSListPath   = port.KHSListPath
	KHSDetailPath = port.KHSDetailPath
)
