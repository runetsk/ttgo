import { test, expect } from '@playwright/test';
import {
    createFolderAPI, createTestAPI, createDefectAPI, updateDefectAPI,
    linkTestCaseDefectAPI, createRequirementAPI, linkRequirementToTestCaseAPI,
} from '../../helpers/api.js';

test.describe('Test case page — linked active bugs & requirements', () => {
    test('shows open bugs (navigable), hides closed, links requirements', async ({ page, request }) => {
        const stamp = Date.now();
        let tc, openBug, closedBug, req;

        await test.step('Seed test case + open bug + closed bug + requirement', async () => {
            const folder = await createFolderAPI(request, `Bug Folder ${stamp}`);
            tc = await createTestAPI(request, `Bug Case ${stamp}`, folder.id);
            openBug = await createDefectAPI(request, { title: `Open bug ${stamp}`, severity: 'major' });
            closedBug = await createDefectAPI(request, { title: `Closed bug ${stamp}`, severity: 'minor' });
            await updateDefectAPI(request, closedBug.id, { status: 'closed' });
            await linkTestCaseDefectAPI(request, tc.id, openBug.id);
            await linkTestCaseDefectAPI(request, tc.id, closedBug.id);
            req = await createRequirementAPI(request, `REQ-${stamp}`, `Requirement ${stamp}`);
            await linkRequirementToTestCaseAPI(request, req.id, tc.id);
        });

        await test.step('Open bug is shown and points at the register; closed bug is hidden', async () => {
            await page.goto(`/library/tests/${tc.id}`);
            await expect(page.getByTestId('linked-bugs-panel')).toBeVisible();
            const bugLink = page.getByTestId(`linked-bug-${openBug.id}`);
            await expect(bugLink).toContainText(`Open bug ${stamp}`);
            await expect(bugLink).toHaveAttribute('href', new RegExp(`/defects\\?focus=${openBug.id}`));
            await expect(page.getByTestId(`linked-bug-${closedBug.id}`)).toHaveCount(0);
        });

        await test.step('Clicking the bug opens the focused Defects register', async () => {
            await page.getByTestId(`linked-bug-${openBug.id}`).click();
            await expect(page).toHaveURL(new RegExp(`/defects\\?focus=${openBug.id}`));
        });

        await test.step('Requirement chip navigates to the requirement detail page', async () => {
            await page.goto(`/library/tests/${tc.id}`);
            const reqLink = page.getByTestId(`linked-req-link-${req.id}`);
            await expect(reqLink).toBeVisible();
            await reqLink.click();
            await expect(page).toHaveURL(new RegExp(`/requirements/${req.id}`));
        });
    });
});
