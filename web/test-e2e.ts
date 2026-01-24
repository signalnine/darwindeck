import { chromium } from 'playwright';

async function runTests() {
  console.log('Starting E2E tests...');

  const browser = await chromium.launch({
    headless: true,
    executablePath: process.env.HOME + '/.cache/ms-playwright/chromium-1208/chrome-linux64/chrome'
  });

  const context = await browser.newContext();
  const page = await context.newPage();

  // Capture console messages
  const consoleMessages: string[] = [];
  page.on('console', msg => {
    consoleMessages.push(`[${msg.type()}] ${msg.text()}`);
  });

  // Capture errors
  const errors: string[] = [];
  page.on('pageerror', error => {
    errors.push(error.message);
  });

  try {
    // Test 1: Load homepage
    console.log('\n=== Test 1: Homepage loads ===');
    await page.goto('http://localhost:5173');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(3000);

    const title = await page.title();
    console.log(`Page title: ${title}`);

    if (title.includes('DarwinDeck')) {
      console.log('✅ Test 1 PASSED: Homepage loaded with correct title');
    } else {
      console.log('❌ Test 1 FAILED: Unexpected title');
    }

    // Test 2: Check for games content
    console.log('\n=== Test 2: Games displayed ===');
    const pageContent = await page.textContent('body');
    const hasInnerWave = pageContent?.includes('InnerWave');
    const hasFirstLynx = pageContent?.includes('FirstLynx');

    if (hasInnerWave && hasFirstLynx) {
      console.log('✅ Test 2 PASSED: Games are displayed');
    } else {
      console.log('❌ Test 2 FAILED: Games not showing');
    }

    // Take screenshot
    await page.screenshot({ path: 'test-01-homepage.png', fullPage: true });
    console.log('Screenshot saved: test-01-homepage.png');

    // Test 3: Click on InnerWave game
    console.log('\n=== Test 3: Click on game ===');
    const gameLink = await page.$('a[href="/game/InnerWave"]');
    if (gameLink) {
      await gameLink.click();
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(3000);

      const gameUrl = page.url();
      console.log(`Game URL: ${gameUrl}`);

      if (gameUrl.includes('/game/InnerWave')) {
        console.log('✅ Test 3 PASSED: Navigated to game page');
      } else {
        console.log('❌ Test 3 FAILED: Wrong URL');
      }

      // Take screenshot
      await page.screenshot({ path: 'test-02-game-page.png', fullPage: true });
      console.log('Screenshot saved: test-02-game-page.png');

      // Check for console errors
      if (errors.length > 0) {
        console.log('\n=== Page errors ===');
        errors.forEach(err => console.log('ERROR:', err));
      }

      // Test 4: Check game page content
      console.log('\n=== Test 4: Game page content ===');
      const gameContent = await page.textContent('body');
      console.log('Game page content preview:', gameContent?.substring(0, 500));

      // Check for game UI elements
      const hasHand = gameContent?.toLowerCase().includes('hand') || gameContent?.toLowerCase().includes('card');
      const hasTurn = gameContent?.toLowerCase().includes('turn');
      const hasLoading = gameContent?.toLowerCase().includes('loading') || gameContent?.toLowerCase().includes('starting');
      const hasError = gameContent?.toLowerCase().includes('error') || gameContent?.toLowerCase().includes('failed');

      console.log(`Has hand/card: ${hasHand}`);
      console.log(`Has turn: ${hasTurn}`);
      console.log(`Has loading: ${hasLoading}`);
      console.log(`Has error: ${hasError}`);

      if (hasError) {
        console.log('⚠️ Test 4: Game page has error state');
      } else if (hasLoading) {
        console.log('⚠️ Test 4: Game is still loading - waiting more');
        await page.waitForTimeout(5000);
        await page.screenshot({ path: 'test-03-game-after-wait.png', fullPage: true });
      } else if (hasHand || hasTurn) {
        console.log('✅ Test 4 PASSED: Game UI elements present');
      } else {
        console.log('⚠️ Test 4: Game page structure unclear');
      }
    } else {
      console.log('❌ Test 3 FAILED: Could not find game link');
    }

    // Print all console messages
    if (consoleMessages.length > 0) {
      console.log('\n=== Console messages ===');
      consoleMessages.forEach(msg => console.log(msg));
    }

    console.log('\n=== All tests complete ===');

  } catch (error) {
    console.error('Test error:', error);
    await page.screenshot({ path: 'test-error.png', fullPage: true });
    console.log('Error screenshot saved: test-error.png');
  } finally {
    await browser.close();
  }
}

runTests().catch(console.error);
