// ==UserScript==
// @name         TMD Download Button
// @namespace    https://github.com/tmd-tool/tmd
// @version      1.2.0
// @description  在 Twitter/X 个人资料页面的"更多"按钮左边添加 TMD 下载按钮，一键推送下载任务
// @author       TMD
// @match        https://x.com/*
// @match        https://twitter.com/*
// @grant        GM_xmlhttpRequest
// @grant        GM_getValue
// @grant        GM_setValue
// @grant        GM_registerMenuCommand
// @connect      localhost
// @connect      127.0.0.1
// @run-at       document-idle
// @license      MIT
// ==/UserScript==

(function () {
    'use strict';

    const DEFAULT_API_URL = 'http://localhost:25556';
    const TMD_BTN_ID = 'tmd-download-btn';
    const INJECT_RETRY_INTERVAL = 1000;
    const INJECT_MAX_RETRIES = 15;

    function getApiUrl() {
        return GM_getValue('tmd_api_url', DEFAULT_API_URL);
    }

    GM_registerMenuCommand('🔧 设置 TMD API 地址', () => {
        const current = getApiUrl();
        const result = prompt('请输入 TMD API 地址：', current);
        if (result !== null && result.trim()) {
            GM_setValue('tmd_api_url', result.trim().replace(/\/+$/, ''));
        }
    });

    function getScreenName() {
        const path = window.location.pathname;
        const match = path.match(/^\/([a-zA-Z0-9_]{1,15})(?:\/|$|\?)/);
        if (match) {
            const name = match[1];
            const reserved = ['home', 'explore', 'search', 'notifications', 'messages', 'settings', 'i', 'compose', 'lists', 'bookmarks', 'communities', 'jobs'];
            if (!reserved.includes(name.toLowerCase())) {
                return name;
            }
        }
        return null;
    }

    function isDarkMode() {
        return document.documentElement.getAttribute('data-color-mode') === 'dark'
            || document.body?.getAttribute('data-color-mode') === 'dark'
            || document.querySelector('meta[name="theme-color"]')?.content === 'rgb(0,0,0)'
            || window.matchMedia('(prefers-color-scheme: dark)').matches;
    }

    let styleEl = null;

    function getThemeColors() {
        const dark = isDarkMode();
        return {
            btnDefault: '#1d9bf0',
            btnLoading: dark ? '#536471' : '#8b98a5',
            btnSuccess: '#00ba7c',
            btnError: '#f4212e',
            toastSuccess: dark ? 'rgba(0,186,124,0.9)' : 'rgba(0,186,124,0.95)',
            toastError: dark ? 'rgba(244,33,46,0.9)' : 'rgba(244,33,46,0.95)',
            toastInfo: dark ? 'rgba(29,155,240,0.9)' : 'rgba(29,155,240,0.95)',
        };
    }

    function buildStyleCSS(colors) {
        return `
            .tmd-dl-btn {
                display: inline-flex;
                align-items: center;
                justify-content: center;
                gap: 4px;
                min-height: 36px;
                padding: 0 16px;
                border-radius: 9999px;
                font-size: 14px;
                font-weight: 700;
                font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
                cursor: pointer;
                border: none;
                outline: none;
                transition: opacity 0.2s, background-color 0.2s;
                white-space: nowrap;
                user-select: none;
                line-height: 20px;
            }
            .tmd-dl-btn:hover { opacity: 0.8; }
            .tmd-dl-btn:active { opacity: 0.6; }
            .tmd-dl-btn--default { background-color: ${colors.btnDefault}; color: #ffffff; }
            .tmd-dl-btn--loading { background-color: ${colors.btnLoading}; color: #ffffff; cursor: wait; }
            .tmd-dl-btn--success { background-color: ${colors.btnSuccess}; color: #ffffff; }
            .tmd-dl-btn--error { background-color: ${colors.btnError}; color: #ffffff; }
            .tmd-dl-btn svg { width: 16px; height: 16px; fill: currentColor; vertical-align: middle; }
            .tmd-dl-toast {
                position: fixed;
                bottom: 24px;
                left: 50%;
                transform: translateX(-50%);
                padding: 10px 20px;
                border-radius: 8px;
                font-size: 14px;
                font-weight: 600;
                font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
                z-index: 99999;
                opacity: 0;
                transition: opacity 0.3s;
                pointer-events: none;
                max-width: 90vw;
                text-align: center;
            }
            .tmd-dl-toast--visible { opacity: 1; }
            .tmd-dl-toast--success { background-color: ${colors.toastSuccess}; color: #fff; }
            .tmd-dl-toast--error { background-color: ${colors.toastError}; color: #fff; }
            .tmd-dl-toast--info { background-color: ${colors.toastInfo}; color: #fff; }
        `;
    }

    function updateStyle() {
        const colors = getThemeColors();
        const css = buildStyleCSS(colors);
        if (styleEl) {
            styleEl.textContent = css;
        } else {
            styleEl = document.createElement('style');
            styleEl.textContent = css;
            document.head.appendChild(styleEl);
        }
    }

    function initStyle() {
        updateStyle();

        const themeObserver = new MutationObserver(() => {
            updateStyle();
        });
        themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['data-color-mode'] });

        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
            updateStyle();
        });
    }

    const DOWNLOAD_ICON = `<svg viewBox="0 0 24 24"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 14v-4H7l5-7 5 7h-4v4h-2z"/></svg>`;

    const CHECK_ICON = `<svg viewBox="0 0 24 24"><path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/></svg>`;

    function showToast(message, type = 'info', duration = 3000) {
        let container = document.getElementById('tmd-toast-container');
        if (!container) {
            container = document.createElement('div');
            container.id = 'tmd-toast-container';
            document.body.appendChild(container);
        }

        const toast = document.createElement('div');
        toast.className = `tmd-dl-toast tmd-dl-toast--${type}`;
        toast.textContent = message;
        container.appendChild(toast);

        requestAnimationFrame(() => {
            toast.classList.add('tmd-dl-toast--visible');
        });

        setTimeout(() => {
            toast.classList.remove('tmd-dl-toast--visible');
            setTimeout(() => toast.remove(), 300);
        }, duration);
    }

    function setButtonState(btn, state, text) {
        btn.className = `tmd-dl-btn tmd-dl-btn--${state}`;
        const icon = state === 'success' ? CHECK_ICON : DOWNLOAD_ICON;
        btn.innerHTML = icon + (text ? ` ${text}` : '');
    }

    async function sendDownload(screenName) {
        const apiUrl = getApiUrl();
        const url = `${apiUrl}/api/v1/users/${encodeURIComponent(screenName)}/download`;

        return new Promise((resolve, reject) => {
            GM_xmlhttpRequest({
                method: 'POST',
                url: url,
                headers: { 'Content-Type': 'application/json' },
                data: JSON.stringify({ auto_follow: true }),
                timeout: 10000,
                onload(response) {
                    try {
                        const data = JSON.parse(response.responseText);
                        if (response.status >= 200 && response.status < 300 && data.success) {
                            resolve(data);
                        } else {
                            reject(new Error(data.message || data.error || `HTTP ${response.status}`));
                        }
                    } catch {
                        reject(new Error(`解析响应失败 (HTTP ${response.status})`));
                    }
                },
                onerror() {
                    reject(new Error('无法连接 TMD 服务，请确认服务已启动'));
                },
                ontimeout() {
                    reject(new Error('请求超时，请确认 TMD 服务可访问'));
                }
            });
        });
    }

    function createDownloadButton(screenName) {
        const btn = document.createElement('button');
        btn.id = TMD_BTN_ID;
        btn.className = 'tmd-dl-btn tmd-dl-btn--default';
        btn.innerHTML = DOWNLOAD_ICON + ' 推送下载';
        btn.title = `推送 @${screenName} 到 TMD 下载队列`;

        btn.addEventListener('click', async (e) => {
            e.preventDefault();
            e.stopPropagation();

            if (btn.classList.contains('tmd-dl-btn--loading')) return;

            const currentName = getScreenName();
            if (!currentName) {
                showToast('无法获取当前用户名', 'error');
                return;
            }

            setButtonState(btn, 'loading', '推送中...');
            try {
                const result = await sendDownload(currentName);
                setButtonState(btn, 'success', '已推送');
                showToast(`已推送 @${currentName} 到下载队列 (任务ID: ${result.data?.task_id || '-'})`, 'success');
                setTimeout(() => {
                    if (document.getElementById(TMD_BTN_ID)) {
                        setButtonState(btn, 'default', '推送下载');
                    }
                }, 3000);
            } catch (err) {
                setButtonState(btn, 'error', '失败');
                showToast(err.message, 'error');
                setTimeout(() => {
                    if (document.getElementById(TMD_BTN_ID)) {
                        setButtonState(btn, 'default', '推送下载');
                    }
                }, 3000);
            }
        });

        return btn;
    }

    function btnExistsInDOM() {
        const btn = document.getElementById(TMD_BTN_ID);
        return btn && btn.isConnected;
    }

    function findMoreButton() {
        const primary = document.querySelector('[data-testid="primaryColumn"]');
        if (!primary) return null;

        // 只在用户个人资料页面注入（有 UserName 区域）
        const userNameArea = primary.querySelector('[data-testid="UserName"]');
        if (!userNameArea) return null;

        // 优先找 userActions（更可靠的标识）
        const userActions = primary.querySelector('[data-testid="userActions"]');
        if (userActions) return userActions;

        // 备选：通过 SVG path 找，但必须靠近 UserName 区域
        const actionItems = primary.querySelectorAll('[role="button"]');
        for (const item of actionItems) {
            // 检查是否在 UserName 附近（同层或父层包含 UserName）
            const nearUserName = item.closest('[data-testid="UserName"]')
                || item.parentElement?.querySelector('[data-testid="UserName"]')
                || item.parentElement?.parentElement?.querySelector('[data-testid="UserName"]');
            if (!nearUserName) continue;

            const svg = item.querySelector('svg');
            if (svg) {
                const pathD = svg.querySelector('path')?.getAttribute('d') || '';
                // 三个点的 SVG path（大小写不敏感，Twitter 使用小写 m）
                if (pathD.toLowerCase().includes('m3 12') && pathD.includes('12c0-1.1')) {
                    const ariaLabel = item.getAttribute('aria-label') || '';
                    const title = item.getAttribute('title') || '';
                    const lower = (ariaLabel + ' ' + title).toLowerCase();
                    if (lower.includes('more') || lower.includes('更多')) {
                        return item;
                    }
                }
            }
        }

        return null;
    }

    let retryTimer = null;
    let retryCount = 0;

    function removeExistingBtn() {
        const existing = document.getElementById(TMD_BTN_ID);
        if (existing) existing.remove();
        const wrapper = document.querySelector('.tmd-dl-wrapper');
        if (wrapper) wrapper.remove();
    }

    function tryInject() {
        const screenName = getScreenName();
        if (!screenName) {
            removeExistingBtn();
            return;
        }

        if (btnExistsInDOM()) return;

        const moreBtn = findMoreButton();
        if (!moreBtn) return;

        removeExistingBtn();

        const btn = createDownloadButton(screenName);

        const wrapper = document.createElement('div');
        wrapper.className = 'tmd-dl-wrapper';
        wrapper.style.display = 'inline-flex';
        wrapper.style.alignItems = 'center';
        wrapper.style.marginRight = '8px';
        wrapper.appendChild(btn);

        const moreWrapper = moreBtn.closest('div') || moreBtn;
        const parent = moreWrapper.parentElement;
        if (!parent) return;
        parent.insertBefore(wrapper, moreWrapper);

        retryCount = 0;
    }

    function scheduleRetry() {
        const screenName = getScreenName();
        if (!screenName) {
            retryCount = 0;
            clearTimeout(retryTimer);
            return;
        }

        if (btnExistsInDOM()) {
            retryCount = 0;
            clearTimeout(retryTimer);
            return;
        }

        if (retryCount >= INJECT_MAX_RETRIES) {
            retryCount = 0;
            return;
        }

        retryCount++;
        clearTimeout(retryTimer);
        retryTimer = setTimeout(() => {
            tryInject();
            scheduleRetry();
        }, INJECT_RETRY_INTERVAL);
    }

    function onURLChange() {
        retryCount = 0;
        clearTimeout(retryTimer);
        removeExistingBtn();

        setTimeout(() => {
            tryInject();
            scheduleRetry();
        }, 300);
    }

    function hookHistoryAPI() {
        const origPushState = history.pushState;
        history.pushState = function () {
            origPushState.apply(this, arguments);
            onURLChange();
        };

        const origReplaceState = history.replaceState;
        history.replaceState = function () {
            origReplaceState.apply(this, arguments);
            onURLChange();
        };

        window.addEventListener('popstate', onURLChange);
    }

    let observerDebounce = null;
    let domObserver = null;

    function cleanup() {
        clearTimeout(retryTimer);
        clearTimeout(observerDebounce);
        if (domObserver) domObserver.disconnect();
        removeExistingBtn();
    }

    function init() {
        initStyle();

        hookHistoryAPI();

        domObserver = new MutationObserver(() => {
            clearTimeout(observerDebounce);
            observerDebounce = setTimeout(() => {
                if (!btnExistsInDOM()) {
                    tryInject();
                }
            }, 200);
        });

        domObserver.observe(document.body, {
            childList: true,
            subtree: true,
        });

        tryInject();
        scheduleRetry();

        window.addEventListener('beforeunload', cleanup, { once: true });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
