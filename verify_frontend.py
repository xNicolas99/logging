from playwright.sync_api import sync_playwright
import time

def verify_frontend():
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()

        # 1. Load Dashboard
        page.goto("http://localhost:8080")
        page.wait_for_selector("#targets-container")

        # 2. Check tiles
        page.wait_for_selector("#targets-container > div")

        # 3. Open Modal for Google
        # Use exact match for heading to avoid matching other tiles if they contained "Google" (though unlikely here)
        # We find the heading "Google", then go up to the card container
        google_heading = page.get_by_role("heading", name="Google", exact=True)

        # The button is in the same card.
        # Structure:
        # Card ->
        #   div (p-5) -> heading
        #   div (bg-gray-50) -> button

        # We can just click the button inside the card that contains the heading
        # Filter tiles (div with shadow) that contain the heading
        google_card = page.locator("#targets-container > div").filter(has=google_heading)

        view_history_btn = google_card.get_by_role("button", name="View History")
        view_history_btn.click()

        # 4. Wait for Modal
        page.wait_for_selector("#detailModal")
        time.sleep(1) # Wait for animation/data load

        # 5. Check Modal Content
        if not page.is_visible("#speedChart"):
            print("Chart not visible!")
            exit(1)

        print("Chart visible.")

        # Check tabs
        if not page.get_by_text("8h").is_visible():
            print("Tabs not visible!")
            exit(1)

        print("Tabs visible.")

        # 6. Take Screenshot of Modal
        page.screenshot(path="/home/jules/verification/verification_final.png")
        print("Screenshot saved.")

        browser.close()

if __name__ == "__main__":
    import os
    os.makedirs("/home/jules/verification", exist_ok=True)
    verify_frontend()
