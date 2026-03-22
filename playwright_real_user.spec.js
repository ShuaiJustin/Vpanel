const { test, expect } = require('@playwright/test');

test('portal smoke', async ({ page }) => {
  await page.goto('http://127.0.0.1:8080/user/login');
  await expect(page).toHaveURL(/\/user\/login$/);
});
