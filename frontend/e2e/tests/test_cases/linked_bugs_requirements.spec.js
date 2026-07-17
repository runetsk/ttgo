import { test, expect } from '../../fixtures/test.js';

test.describe('Test case page — linked active bugs & requirements', () => {
    test('shows open bugs (navigable), hides closed, links requirements', async ({ page, api, testCaseDetailPage }) => {
        const stamp = Date.now();
        let tc, openBug, closedBug, req;

        await test.step('Seed test case + open bug + closed bug + requirement', async () => {
            const folder = await api.createFolder(`Bug Folder ${stamp}`);
            tc = await api.createTest(`Bug Case ${stamp}`, folder.id);
            openBug = await api.createDefect({ title: `Open bug ${stamp}`, severity: 'major' });
            closedBug = await api.createDefect({ title: `Closed bug ${stamp}`, severity: 'minor' });
            await api.updateDefect(closedBug.id, { status: 'closed' });
            await api.linkTestCaseDefect(tc.id, openBug.id);
            await api.linkTestCaseDefect(tc.id, closedBug.id);
            req = await api.createRequirement(`REQ-${stamp}`, `Requirement ${stamp}`);
            await api.linkRequirementToTestCase(req.id, tc.id);
        });

        await test.step('Open bug is shown and points at the register; closed bug is hidden', async () => {
            await testCaseDetailPage.open(tc.id);
            await expect(testCaseDetailPage.linkedBugsPanel).toBeVisible();
            const bugLink = testCaseDetailPage.linkedBug(openBug.id);
            await expect(bugLink).toContainText(`Open bug ${stamp}`);
            await expect(bugLink).toHaveAttribute('href', new RegExp(`/defects\\?focus=${openBug.id}`));
            await expect(testCaseDetailPage.linkedBug(closedBug.id)).toHaveCount(0);
        });

        await test.step('Clicking the bug opens the focused Defects register', async () => {
            await testCaseDetailPage.linkedBug(openBug.id).click();
            await expect(page).toHaveURL(new RegExp(`/defects\\?focus=${openBug.id}`));
        });

        await test.step('Requirement chip navigates to the requirement detail page', async () => {
            await testCaseDetailPage.open(tc.id);
            const reqLink = testCaseDetailPage.linkedReqLink(req.id);
            await expect(reqLink).toBeVisible();
            await reqLink.click();
            await expect(page).toHaveURL(new RegExp(`/requirements/${req.id}`));
        });
    });
});
