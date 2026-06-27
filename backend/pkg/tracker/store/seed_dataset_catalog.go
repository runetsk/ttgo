package store

import (
	"time"
	"ttgo/pkg/tracker/models"
)

type demoTestCaseRef struct {
	ID   string
	Name string
}

type demoCatalogData struct {
	Folders             []models.Folder
	Categories          []models.Category
	TestCases           []models.TestCase
	Steps               []models.TestStep
	CategoryAssignments []models.CategoryTestCase
	CategoryIDs         map[string]string
	TestCasesByKey      map[string]demoTestCaseRef
}

func buildDemoCatalog(now time.Time) demoCatalogData {
	rootID := demoID("folder:ecommerce-app")
	authID := demoID("folder:authentication")
	checkoutID := demoID("folder:checkout")
	searchID := demoID("folder:search-browse")

	folders := []models.Folder{
		{ID: rootID, Name: "E-Commerce App", ParentID: nil, CreatedAt: now, UpdatedAt: now},
		{ID: authID, Name: "Authentication", ParentID: &rootID, CreatedAt: now, UpdatedAt: now},
		{ID: checkoutID, Name: "Checkout", ParentID: &rootID, CreatedAt: now, UpdatedAt: now},
		{ID: searchID, Name: "Search & Browse", ParentID: &rootID, CreatedAt: now, UpdatedAt: now},
	}

	smokeID := demoID("category:smoke")
	regressionID := demoID("category:regression")
	edgeID := demoID("category:edge-cases")

	categories := []models.Category{
		{ID: smokeID, Name: "Smoke", Description: "Critical path tests run on every deploy", CreatedAt: now, UpdatedAt: now},
		{ID: regressionID, Name: "Regression", Description: "Full regression coverage", CreatedAt: now, UpdatedAt: now},
		{ID: edgeID, Name: "Edge Cases", Description: "Boundary conditions and error paths", CreatedAt: now, UpdatedAt: now},
	}

	testCasesByKey := map[string]demoTestCaseRef{
		"tc1":  {ID: demoID("tc:homepage-loads"), Name: "Homepage loads within expected time"},
		"tc2":  {ID: demoID("tc:nav-links"), Name: "Main navigation links are functional"},
		"tc3":  {ID: demoID("tc:seo-meta-tags"), Name: "SEO meta tags are present on key pages"},
		"tc4":  {ID: demoID("tc:404-page"), Name: "Custom 404 page is shown for unknown routes"},
		"tc5":  {ID: demoID("tc:login-valid"), Name: "User can log in with valid credentials"},
		"tc6":  {ID: demoID("tc:login-wrong-password"), Name: "Login fails with incorrect password"},
		"tc7":  {ID: demoID("tc:login-empty-email"), Name: "Login fails with empty email"},
		"tc8":  {ID: demoID("tc:logout"), Name: "User can log out successfully"},
		"tc9":  {ID: demoID("tc:session-expires"), Name: "Session expires after inactivity"},
		"tc10": {ID: demoID("tc:password-reset"), Name: "Password reset email is delivered"},
		"tc11": {ID: demoID("tc:account-lockout"), Name: "Account lockout after repeated failures"},
		"tc12": {ID: demoID("tc:concurrent-sessions"), Name: "Concurrent session handling"},
		"tc13": {ID: demoID("tc:add-to-cart"), Name: "User can add item to cart"},
		"tc14": {ID: demoID("tc:remove-from-cart"), Name: "User can remove item from cart"},
		"tc15": {ID: demoID("tc:checkout-valid-payment"), Name: "Checkout completes with valid payment"},
		"tc16": {ID: demoID("tc:empty-cart-message"), Name: "Empty cart shows appropriate message"},
		"tc17": {ID: demoID("tc:coupon-code"), Name: "Coupon code is applied correctly"},
		"tc18": {ID: demoID("tc:out-of-stock"), Name: "Out-of-stock item cannot be purchased"},
		"tc19": {ID: demoID("tc:order-confirmation-email"), Name: "Order confirmation email is sent"},
		"tc20": {ID: demoID("tc:keyword-search"), Name: "Keyword search returns relevant results"},
		"tc21": {ID: demoID("tc:category-filter"), Name: "Category filter narrows results correctly"},
		"tc22": {ID: demoID("tc:empty-search"), Name: "Empty search query shows all products"},
		"tc23": {ID: demoID("tc:pagination"), Name: "Pagination loads correct page of results"},
		"tc24": {ID: demoID("tc:special-chars"), Name: "Search with special characters handles gracefully"},
	}

	testCases := []models.TestCase{
		{ID: testCasesByKey["tc1"].ID, FolderID: rootID, Name: testCasesByKey["tc1"].Name, Description: "Verify the homepage renders and is interactive within the expected time threshold.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc2"].ID, FolderID: rootID, Name: testCasesByKey["tc2"].Name, Description: "All links in the primary navigation lead to the correct pages.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc3"].ID, FolderID: rootID, Name: testCasesByKey["tc3"].Name, Description: "Title, description, and og: tags exist and are populated on homepage and product pages.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc4"].ID, FolderID: rootID, Name: testCasesByKey["tc4"].Name, Description: "Navigating to a non-existent URL displays the custom 404 error page.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc5"].ID, FolderID: authID, Name: testCasesByKey["tc5"].Name, Description: "Registered user submits correct email and password and lands on the dashboard.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc6"].ID, FolderID: authID, Name: testCasesByKey["tc6"].Name, Description: "Submitting a wrong password shows an appropriate error and does not authenticate.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc7"].ID, FolderID: authID, Name: testCasesByKey["tc7"].Name, Description: "Submitting a blank email field is rejected with a validation message.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc8"].ID, FolderID: authID, Name: testCasesByKey["tc8"].Name, Description: "Clicking Logout clears the session and redirects to the login page.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc9"].ID, FolderID: authID, Name: testCasesByKey["tc9"].Name, Description: "An idle session is automatically invalidated after the configured timeout period.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc10"].ID, FolderID: authID, Name: testCasesByKey["tc10"].Name, Description: "Requesting a password reset sends a reset link to the registered email address.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc11"].ID, FolderID: authID, Name: testCasesByKey["tc11"].Name, Description: "The account is temporarily locked after exceeding the allowed number of failed login attempts.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc12"].ID, FolderID: authID, Name: testCasesByKey["tc12"].Name, Description: "Logging in from a second device either invalidates the first session or coexists per policy.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc13"].ID, FolderID: checkoutID, Name: testCasesByKey["tc13"].Name, Description: "Clicking 'Add to Cart' on a product page increments the cart item count.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc14"].ID, FolderID: checkoutID, Name: testCasesByKey["tc14"].Name, Description: "Removing an item from the cart correctly updates the cart total and item list.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc15"].ID, FolderID: checkoutID, Name: testCasesByKey["tc15"].Name, Description: "Submitting a valid payment method results in an order confirmation screen.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc16"].ID, FolderID: checkoutID, Name: testCasesByKey["tc16"].Name, Description: "Visiting the cart page with no items displays a friendly empty-state message.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc17"].ID, FolderID: checkoutID, Name: testCasesByKey["tc17"].Name, Description: "Entering a valid coupon code reduces the order total by the expected discount amount.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc18"].ID, FolderID: checkoutID, Name: testCasesByKey["tc18"].Name, Description: "The Add to Cart button is disabled for out-of-stock products.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc19"].ID, FolderID: checkoutID, Name: testCasesByKey["tc19"].Name, Description: "After a successful purchase, the customer receives an order confirmation email.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc20"].ID, FolderID: searchID, Name: testCasesByKey["tc20"].Name, Description: "Searching by a product keyword returns results that match the query term.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc21"].ID, FolderID: searchID, Name: testCasesByKey["tc21"].Name, Description: "Selecting a category filter reduces the result set to matching products only.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc22"].ID, FolderID: searchID, Name: testCasesByKey["tc22"].Name, Description: "Submitting an empty search term displays the full product catalogue.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc23"].ID, FolderID: searchID, Name: testCasesByKey["tc23"].Name, Description: "Navigating to page 2 shows a different set of products than page 1.", CreatedAt: now, UpdatedAt: now},
		{ID: testCasesByKey["tc24"].ID, FolderID: searchID, Name: testCasesByKey["tc24"].Name, Description: "Searching for terms with special characters (e.g., &, %, <) does not cause errors.", CreatedAt: now, UpdatedAt: now},
	}

	steps := []models.TestStep{
		{ID: demoID("step:tc1-1"), TestCaseID: testCasesByKey["tc1"].ID, Action: "Navigate to the homepage URL", ExpectedResult: "Page loads without errors", OrderIndex: 0},
		{ID: demoID("step:tc1-2"), TestCaseID: testCasesByKey["tc1"].ID, Action: "Measure time to interactive", ExpectedResult: "Time to interactive is within the defined threshold", OrderIndex: 1},
		{ID: demoID("step:tc2-1"), TestCaseID: testCasesByKey["tc2"].ID, Action: "Click each link in the main navigation bar", ExpectedResult: "Each link navigates to the corresponding page without 404 errors", OrderIndex: 0},
		{ID: demoID("step:tc2-2"), TestCaseID: testCasesByKey["tc2"].ID, Action: "Verify the active state on the current nav item", ExpectedResult: "The active nav item is visually highlighted", OrderIndex: 1},
		{ID: demoID("step:tc3-1"), TestCaseID: testCasesByKey["tc3"].ID, Action: "Open the homepage in a browser", ExpectedResult: "Page renders successfully", OrderIndex: 0},
		{ID: demoID("step:tc3-2"), TestCaseID: testCasesByKey["tc3"].ID, Action: "Inspect <title>, <meta name='description'>, and og:title tags", ExpectedResult: "All three tags are present and contain non-empty values", OrderIndex: 1},
		{ID: demoID("step:tc4-1"), TestCaseID: testCasesByKey["tc4"].ID, Action: "Navigate to /this-route-does-not-exist", ExpectedResult: "HTTP 404 status is returned", OrderIndex: 0},
		{ID: demoID("step:tc4-2"), TestCaseID: testCasesByKey["tc4"].ID, Action: "Inspect the rendered page content", ExpectedResult: "Custom 404 page with a helpful message is displayed", OrderIndex: 1},
		{ID: demoID("step:tc5-1"), TestCaseID: testCasesByKey["tc5"].ID, Action: "Navigate to the login page", ExpectedResult: "Login form is visible", OrderIndex: 0},
		{ID: demoID("step:tc5-2"), TestCaseID: testCasesByKey["tc5"].ID, Action: "Enter valid email and password and submit", ExpectedResult: "User is redirected to the dashboard", OrderIndex: 1},
		{ID: demoID("step:tc5-3"), TestCaseID: testCasesByKey["tc5"].ID, Action: "Confirm authenticated state", ExpectedResult: "User name appears in the header", OrderIndex: 2},
		{ID: demoID("step:tc6-1"), TestCaseID: testCasesByKey["tc6"].ID, Action: "Submit the login form with a wrong password", ExpectedResult: "An error message is displayed", OrderIndex: 0},
		{ID: demoID("step:tc6-2"), TestCaseID: testCasesByKey["tc6"].ID, Action: "Verify user remains on the login page", ExpectedResult: "URL has not changed from the login page", OrderIndex: 1},
		{ID: demoID("step:tc7-1"), TestCaseID: testCasesByKey["tc7"].ID, Action: "Submit the login form with the email field blank", ExpectedResult: "Validation error appears for the email field", OrderIndex: 0},
		{ID: demoID("step:tc7-2"), TestCaseID: testCasesByKey["tc7"].ID, Action: "Verify no server request is made", ExpectedResult: "No network request is sent to the authentication endpoint", OrderIndex: 1},
		{ID: demoID("step:tc8-1"), TestCaseID: testCasesByKey["tc8"].ID, Action: "Log in as a valid user", ExpectedResult: "User is authenticated", OrderIndex: 0},
		{ID: demoID("step:tc8-2"), TestCaseID: testCasesByKey["tc8"].ID, Action: "Click the Logout button", ExpectedResult: "Session is invalidated and user is redirected to the login page", OrderIndex: 1},
		{ID: demoID("step:tc9-1"), TestCaseID: testCasesByKey["tc9"].ID, Action: "Log in and leave the session idle beyond the timeout duration", ExpectedResult: "Session token expires", OrderIndex: 0},
		{ID: demoID("step:tc9-2"), TestCaseID: testCasesByKey["tc9"].ID, Action: "Attempt to perform an authenticated action", ExpectedResult: "User is redirected to the login page with a session-expired message", OrderIndex: 1},
		{ID: demoID("step:tc10-1"), TestCaseID: testCasesByKey["tc10"].ID, Action: "Click 'Forgot Password' on the login page and enter registered email", ExpectedResult: "Confirmation message indicates email has been sent", OrderIndex: 0},
		{ID: demoID("step:tc10-2"), TestCaseID: testCasesByKey["tc10"].ID, Action: "Check the inbox of the registered email", ExpectedResult: "Password reset email is received within 2 minutes", OrderIndex: 1},
		{ID: demoID("step:tc11-1"), TestCaseID: testCasesByKey["tc11"].ID, Action: "Attempt to log in with wrong credentials repeatedly", ExpectedResult: "After the allowed failure count, the account is locked", OrderIndex: 0},
		{ID: demoID("step:tc11-2"), TestCaseID: testCasesByKey["tc11"].ID, Action: "Try to log in with correct credentials while locked", ExpectedResult: "Login is blocked with an account-locked message", OrderIndex: 1},
		{ID: demoID("step:tc12-1"), TestCaseID: testCasesByKey["tc12"].ID, Action: "Log in from device A", ExpectedResult: "Session A is active", OrderIndex: 0},
		{ID: demoID("step:tc12-2"), TestCaseID: testCasesByKey["tc12"].ID, Action: "Log in from device B with the same account", ExpectedResult: "Behaviour matches documented session policy (concurrent or invalidated)", OrderIndex: 1},
		{ID: demoID("step:tc13-1"), TestCaseID: testCasesByKey["tc13"].ID, Action: "Navigate to a product page", ExpectedResult: "Product details and Add to Cart button are visible", OrderIndex: 0},
		{ID: demoID("step:tc13-2"), TestCaseID: testCasesByKey["tc13"].ID, Action: "Click Add to Cart", ExpectedResult: "Cart item count increments by 1", OrderIndex: 1},
		{ID: demoID("step:tc14-1"), TestCaseID: testCasesByKey["tc14"].ID, Action: "Open the cart with at least one item", ExpectedResult: "Item is listed with correct price", OrderIndex: 0},
		{ID: demoID("step:tc14-2"), TestCaseID: testCasesByKey["tc14"].ID, Action: "Click Remove for the item", ExpectedResult: "Item is removed and cart total updates accordingly", OrderIndex: 1},
		{ID: demoID("step:tc15-1"), TestCaseID: testCasesByKey["tc15"].ID, Action: "Add a product to the cart and proceed to checkout", ExpectedResult: "Checkout form is displayed", OrderIndex: 0},
		{ID: demoID("step:tc15-2"), TestCaseID: testCasesByKey["tc15"].ID, Action: "Enter valid payment details and submit the order", ExpectedResult: "Order confirmation page is displayed with an order number", OrderIndex: 1},
		{ID: demoID("step:tc16-1"), TestCaseID: testCasesByKey["tc16"].ID, Action: "Navigate to the cart page with no items in the cart", ExpectedResult: "Empty-state message is displayed, no item list is shown", OrderIndex: 0},
		{ID: demoID("step:tc16-2"), TestCaseID: testCasesByKey["tc16"].ID, Action: "Verify the checkout button is absent or disabled", ExpectedResult: "User cannot proceed to checkout from an empty cart", OrderIndex: 1},
		{ID: demoID("step:tc17-1"), TestCaseID: testCasesByKey["tc17"].ID, Action: "Add an item to the cart and navigate to checkout", ExpectedResult: "Coupon code input field is visible", OrderIndex: 0},
		{ID: demoID("step:tc17-2"), TestCaseID: testCasesByKey["tc17"].ID, Action: "Enter a valid coupon code and apply it", ExpectedResult: "Discount is applied and the order total reflects the reduction", OrderIndex: 1},
		{ID: demoID("step:tc18-1"), TestCaseID: testCasesByKey["tc18"].ID, Action: "Navigate to a product page for an out-of-stock item", ExpectedResult: "Add to Cart button is disabled or replaced with 'Out of Stock' label", OrderIndex: 0},
		{ID: demoID("step:tc18-2"), TestCaseID: testCasesByKey["tc18"].ID, Action: "Attempt to add the item via direct cart manipulation", ExpectedResult: "Server rejects the addition with an appropriate error response", OrderIndex: 1},
		{ID: demoID("step:tc19-1"), TestCaseID: testCasesByKey["tc19"].ID, Action: "Complete a purchase with a valid email address", ExpectedResult: "Order confirmation page is displayed", OrderIndex: 0},
		{ID: demoID("step:tc19-2"), TestCaseID: testCasesByKey["tc19"].ID, Action: "Check the inbox of the email used during checkout", ExpectedResult: "Order confirmation email is received within 5 minutes", OrderIndex: 1},
		{ID: demoID("step:tc20-1"), TestCaseID: testCasesByKey["tc20"].ID, Action: "Enter a product keyword in the search bar and submit", ExpectedResult: "Search results page is displayed", OrderIndex: 0},
		{ID: demoID("step:tc20-2"), TestCaseID: testCasesByKey["tc20"].ID, Action: "Verify results contain items matching the keyword", ExpectedResult: "All displayed results are relevant to the search term", OrderIndex: 1},
		{ID: demoID("step:tc21-1"), TestCaseID: testCasesByKey["tc21"].ID, Action: "On the search results page, select a category filter", ExpectedResult: "Result set is filtered to the selected category only", OrderIndex: 0},
		{ID: demoID("step:tc21-2"), TestCaseID: testCasesByKey["tc21"].ID, Action: "Verify no items from other categories are shown", ExpectedResult: "Every listed product belongs to the selected category", OrderIndex: 1},
		{ID: demoID("step:tc22-1"), TestCaseID: testCasesByKey["tc22"].ID, Action: "Submit an empty search query", ExpectedResult: "All products are displayed without filtering", OrderIndex: 0},
		{ID: demoID("step:tc22-2"), TestCaseID: testCasesByKey["tc22"].ID, Action: "Verify pagination reflects the total product count", ExpectedResult: "Total result count matches the full catalogue size", OrderIndex: 1},
		{ID: demoID("step:tc23-1"), TestCaseID: testCasesByKey["tc23"].ID, Action: "Load page 1 of search results and note the displayed items", ExpectedResult: "First page of results is shown", OrderIndex: 0},
		{ID: demoID("step:tc23-2"), TestCaseID: testCasesByKey["tc23"].ID, Action: "Click 'Next' or page 2 and compare results", ExpectedResult: "A different set of products is displayed on page 2", OrderIndex: 1},
		{ID: demoID("step:tc24-1"), TestCaseID: testCasesByKey["tc24"].ID, Action: "Search for a term containing special characters (e.g., 'men&women')", ExpectedResult: "Results page loads without errors", OrderIndex: 0},
		{ID: demoID("step:tc24-2"), TestCaseID: testCasesByKey["tc24"].ID, Action: "Verify no server error is returned", ExpectedResult: "HTTP 200 status and a valid results page are returned", OrderIndex: 1},
	}

	categoryAssignments := []models.CategoryTestCase{
		{CategoryID: smokeID, TestCaseID: testCasesByKey["tc5"].ID},
		{CategoryID: regressionID, TestCaseID: testCasesByKey["tc5"].ID},
		{CategoryID: regressionID, TestCaseID: testCasesByKey["tc6"].ID},
		{CategoryID: edgeID, TestCaseID: testCasesByKey["tc6"].ID},
		{CategoryID: edgeID, TestCaseID: testCasesByKey["tc7"].ID},
		{CategoryID: smokeID, TestCaseID: testCasesByKey["tc8"].ID},
		{CategoryID: regressionID, TestCaseID: testCasesByKey["tc9"].ID},
		{CategoryID: regressionID, TestCaseID: testCasesByKey["tc10"].ID},
		{CategoryID: edgeID, TestCaseID: testCasesByKey["tc11"].ID},
		{CategoryID: edgeID, TestCaseID: testCasesByKey["tc12"].ID},
		{CategoryID: smokeID, TestCaseID: testCasesByKey["tc13"].ID},
		{CategoryID: regressionID, TestCaseID: testCasesByKey["tc14"].ID},
		{CategoryID: smokeID, TestCaseID: testCasesByKey["tc15"].ID},
		{CategoryID: regressionID, TestCaseID: testCasesByKey["tc15"].ID},
		{CategoryID: edgeID, TestCaseID: testCasesByKey["tc16"].ID},
		{CategoryID: regressionID, TestCaseID: testCasesByKey["tc17"].ID},
		{CategoryID: edgeID, TestCaseID: testCasesByKey["tc18"].ID},
		{CategoryID: regressionID, TestCaseID: testCasesByKey["tc19"].ID},
		{CategoryID: smokeID, TestCaseID: testCasesByKey["tc20"].ID},
		{CategoryID: regressionID, TestCaseID: testCasesByKey["tc21"].ID},
		{CategoryID: edgeID, TestCaseID: testCasesByKey["tc22"].ID},
		{CategoryID: regressionID, TestCaseID: testCasesByKey["tc23"].ID},
		{CategoryID: edgeID, TestCaseID: testCasesByKey["tc24"].ID},
	}

	return demoCatalogData{
		Folders:             folders,
		Categories:          categories,
		TestCases:           testCases,
		Steps:               steps,
		CategoryAssignments: categoryAssignments,
		CategoryIDs: map[string]string{
			"smoke":      smokeID,
			"regression": regressionID,
			"edge":       edgeID,
		},
		TestCasesByKey: testCasesByKey,
	}
}
