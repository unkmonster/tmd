// ==UserScript==
// @name         TMD X Download Button
// @namespace    https://github.com/unkmonster/tmd
// @version      0.1.0
// @description  Add a TMD download button next to the follow actions on X profile pages.
// @author       Codex
// @match        https://x.com/*
// @match        https://twitter.com/*
// @grant        GM_xmlhttpRequest
// @connect      127.0.0.1
// @connect      localhost
// @run-at       document-idle
// ==/UserScript==

(function () {
  'use strict';

  const API_BASE = 'http://127.0.0.1:25556';
  const WRAPPER_CLASS = 'tmd-download-now-wrapper';
  const BUTTON_CLASS = 'tmd-download-now-button';
  const BUTTON_LABEL_CLASS = 'tmd-download-now-button-label';
  const STYLE_ID = 'tmd-download-now-button-style';
  const FOLLOW_BUTTON_SELECTOR = [
    'button[data-testid$="-follow"]',
    'button[data-testid="follow"]',
    'button[data-testid="unfollow"]',
    'button[data-testid="pending"]',
  ].join(', ');
  const OTHER_PROFILE_ACTION_SELECTOR = [
    '[data-testid="userActions"]',
    'button[data-testid="sendDMFromProfile"]',
  ].join(', ');
  const RESERVED_PATHS = new Set([
    'home',
    'explore',
    'notifications',
    'messages',
    'i',
    'search',
    'settings',
    'compose',
    'login',
    'signup',
    'tos',
    'privacy',
    'account',
  ]);
  const PROFILE_SUBPAGES = new Set([
    'media',
    'with_replies',
    'followers',
    'following',
    'verified_followers',
    'highlights',
    'articles',
    'likes',
  ]);

  let renderTimer = null;
  let alignTimer = null;

  function injectStyles() {
    if (document.getElementById(STYLE_ID)) {
      return;
    }

    const style = document.createElement('style');
    style.id = STYLE_ID;
    style.textContent = `
      .${WRAPPER_CLASS} {
        margin-right: 8px !important;
      }

      .${BUTTON_CLASS} {
        appearance: none !important;
        -webkit-appearance: none !important;
        position: relative;
        display: inline-flex !important;
        align-items: center !important;
        justify-content: center !important;
        width: 40px !important;
        min-width: 40px !important;
        height: 40px !important;
        min-height: 40px !important;
        padding: 0 !important;
        margin: 0 !important;
        border: 1px solid rgb(207, 217, 222) !important;
        border-radius: 9999px !important;
        background: rgb(255, 255, 255) !important;
        color: rgb(15, 20, 25) !important;
        box-sizing: border-box !important;
        cursor: pointer !important;
        line-height: 0 !important;
        transition:
          background-color 140ms ease,
          border-color 140ms ease,
          color 140ms ease,
          opacity 140ms ease,
          transform 140ms ease;
      }

      .${BUTTON_CLASS}:hover {
        background: rgb(239, 243, 244) !important;
      }

      .${BUTTON_CLASS} svg {
        width: 20px !important;
        height: 20px !important;
        display: block !important;
        stroke: currentColor !important;
      }

      .${BUTTON_CLASS}[data-state="loading"] {
        opacity: 0.72;
      }

      .${BUTTON_CLASS}[data-state="success"] {
        color: rgb(6, 95, 70);
      }

      .${BUTTON_CLASS}[data-state="error"] {
        color: rgb(185, 28, 28);
      }

      .${BUTTON_CLASS}:disabled {
        cursor: wait;
      }

      .${BUTTON_LABEL_CLASS} {
        position: absolute;
        width: 1px;
        height: 1px;
        padding: 0;
        margin: -1px;
        overflow: hidden;
        clip: rect(0, 0, 0, 0);
        white-space: nowrap;
        border: 0;
      }
    `;

    document.head.appendChild(style);
  }

  function scheduleRender() {
    if (renderTimer) {
      clearTimeout(renderTimer);
    }
    renderTimer = setTimeout(() => {
      renderTimer = null;
      renderButton();
    }, 120);
  }

  function getCurrentScreenName() {
    const parts = window.location.pathname.split('/').filter(Boolean);
    if (parts.length === 0) {
      return null;
    }

    const screenName = parts[0];
    if (!screenName || RESERVED_PATHS.has(screenName.toLowerCase())) {
      return null;
    }

    if (parts.length === 1) {
      return screenName;
    }

    if (parts.length === 2 && PROFILE_SUBPAGES.has(parts[1].toLowerCase())) {
      return screenName;
    }

    return null;
  }

  function findProfileActions() {
    const followButton = document.querySelector(FOLLOW_BUTTON_SELECTOR);
    if (!followButton) {
      return null;
    }

    const targetSlot = findProfileActionSlot(followButton);
    return {
      root: targetSlot ? targetSlot.parentElement : followButton.parentElement,
      targetSlot,
    };
  }

  function findProfileActionSlot(button) {
    if (!button) {
      return null;
    }

    let slot = button;
    while (slot.parentElement) {
      const parent = slot.parentElement;
      const hasFollow = !!parent.querySelector(FOLLOW_BUTTON_SELECTOR);
      const hasOtherAction = !!parent.querySelector(OTHER_PROFILE_ACTION_SELECTOR);

      if (hasFollow && hasOtherAction && parent.children.length >= 2) {
        return slot;
      }

      slot = parent;
    }

    return button;
  }

  function removeStaleButtons(activeScreenName) {
    document.querySelectorAll(`.${WRAPPER_CLASS}`).forEach((wrapper) => {
      const button = wrapper.querySelector(`.${BUTTON_CLASS}`);
      if (!activeScreenName || !button || button.dataset.screenName !== activeScreenName) {
        wrapper.remove();
      }
    });
  }

  function setButtonState(button, label, disabled, state) {
    const labelNode = button.querySelector(`.${BUTTON_LABEL_CLASS}`);
    if (labelNode) {
      labelNode.textContent = label;
    } else {
      button.textContent = label;
    }
    button.disabled = disabled;
    button.dataset.state = state;
    button.title = label;
    button.setAttribute('aria-label', label);
  }

  function requestDownload(screenName, button) {
    const url = `${API_BASE}/api/v1/users/${encodeURIComponent(screenName)}/download`;
    setButtonState(button, '正在推送到 TMD', true, 'loading');

    GM_xmlhttpRequest({
      method: 'POST',
      url,
      headers: {
        'Content-Type': 'application/json',
      },
      data: '{}',
      timeout: 15000,
      onload(response) {
        let payload = null;
        try {
          payload = JSON.parse(response.responseText || '{}');
        } catch (_) {
          payload = null;
        }

        if (response.status >= 200 && response.status < 300 && payload && payload.success) {
          setButtonState(button, '已加入 TMD 下载队列', false, 'success');
          setTimeout(() => {
            if (button.isConnected) {
              setButtonState(button, `推送 ${screenName} 到 TMD 下载队列`, false, 'idle');
            }
          }, 3000);
          return;
        }

        const errMsg = payload && payload.error ? payload.error : `HTTP ${response.status}`;
        setButtonState(button, `推送失败: ${errMsg}`, false, 'error');
      },
      ontimeout() {
        setButtonState(button, '推送失败: 请求超时', false, 'error');
      },
      onerror() {
        setButtonState(button, '推送失败: 无法连接 TMD', false, 'error');
      },
    });
  }

  function createButtonWrapper(screenName, referenceSlot) {
    const wrapper = document.createElement('div');
    if (referenceSlot && referenceSlot.className) {
      wrapper.className = `${referenceSlot.className} ${WRAPPER_CLASS}`;
    } else {
      wrapper.className = WRAPPER_CLASS;
    }
    if (referenceSlot && referenceSlot.getAttribute('style')) {
      wrapper.setAttribute('style', referenceSlot.getAttribute('style'));
    }

    const button = document.createElement('button');
    button.type = 'button';
    button.className = BUTTON_CLASS;
    button.dataset.screenName = screenName;
    button.dataset.state = 'idle';
    button.title = `推送 ${screenName} 到 TMD 下载队列`;
    button.setAttribute('aria-label', button.title);
    button.innerHTML = `
      <svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M12 3v11"></path>
        <path d="m7 10 5 5 5-5"></path>
        <path d="M5 21h14"></path>
      </svg>
      <span class="${BUTTON_LABEL_CLASS}">推送下载</span>
    `;

    button.addEventListener('click', (event) => {
      event.preventDefault();
      event.stopPropagation();
      requestDownload(screenName, button);
    });

    wrapper.appendChild(button);
    return wrapper;
  }

  function findReferenceActionSlot(root, targetSlot) {
    if (!root || !targetSlot) {
      return null;
    }

    const children = Array.from(root.children);
    const targetIndex = children.indexOf(targetSlot);
    for (let index = targetIndex - 1; index >= 0; index--) {
      if (children[index].querySelector('button')) {
        return children[index];
      }
    }

    return null;
  }

  function findPreviousActionButton(wrapper) {
    if (!wrapper) {
      return null;
    }

    let slot = wrapper.previousElementSibling;
    while (slot) {
      const button = slot.querySelector && slot.querySelector('button');
      if (button && !button.closest(`.${WRAPPER_CLASS}`)) {
        return button;
      }
      slot = slot.previousElementSibling;
    }

    return null;
  }

  function alignWrapperByGeometry(wrapper, referenceButton) {
    if (!wrapper || !referenceButton) {
      return;
    }

    const button = wrapper.querySelector(`.${BUTTON_CLASS}`);
    if (!button) {
      return;
    }

    wrapper.style.transform = '';
    requestAnimationFrame(() => {
      if (!wrapper.isConnected || !referenceButton.isConnected) {
        return;
      }

      const referenceRect = referenceButton.getBoundingClientRect();
      const buttonRect = button.getBoundingClientRect();
      if (referenceRect.height === 0 || buttonRect.height === 0) {
        return;
      }

      const referenceCenter = referenceRect.top + referenceRect.height / 2;
      const buttonCenter = buttonRect.top + buttonRect.height / 2;
      const delta = Math.round(referenceCenter - buttonCenter);
      wrapper.style.transform = delta === 0 ? '' : `translateY(${delta}px)`;
    });
  }

  function realignButtons() {
    document.querySelectorAll(`.${WRAPPER_CLASS}`).forEach((wrapper) => {
      const referenceButton = findPreviousActionButton(wrapper);
      alignWrapperByGeometry(wrapper, referenceButton);
    });
  }

  function scheduleRealign() {
    if (alignTimer) {
      clearTimeout(alignTimer);
    }
    alignTimer = setTimeout(() => {
      alignTimer = null;
      realignButtons();
    }, 80);
  }

  function renderButton() {
    injectStyles();

    const screenName = getCurrentScreenName();
    removeStaleButtons(screenName);
    if (!screenName) {
      return;
    }

    const existing = document.querySelector(`.${WRAPPER_CLASS} .${BUTTON_CLASS}[data-screen-name="${screenName}"]`);
    if (existing) {
      return;
    }

    const actions = findProfileActions();
    if (!actions || !actions.root) {
      return;
    }

    const referenceSlot = findReferenceActionSlot(actions.root, actions.targetSlot);
    const referenceButton = referenceSlot ? referenceSlot.querySelector('button') : null;
    const wrapper = createButtonWrapper(screenName, referenceSlot);
    if (actions.targetSlot && actions.targetSlot.parentElement === actions.root) {
      actions.root.insertBefore(wrapper, actions.targetSlot);
    } else {
      actions.root.appendChild(wrapper);
    }

    const alignReferenceButton = actions.targetSlot?.querySelector('button') || referenceButton || findPreviousActionButton(wrapper);
    alignWrapperByGeometry(wrapper, alignReferenceButton);
    setTimeout(() => alignWrapperByGeometry(wrapper, alignReferenceButton), 250);
  }

  const observer = new MutationObserver(() => {
    scheduleRender();
  });

  function patchHistoryMethod(methodName) {
    const original = history[methodName];
    if (typeof original !== 'function') {
      return;
    }

    history[methodName] = function patchedHistoryMethod(...args) {
      const result = original.apply(this, args);
      scheduleRender();
      return result;
    };
  }

  patchHistoryMethod('pushState');
  patchHistoryMethod('replaceState');
  window.addEventListener('popstate', scheduleRender);
  window.addEventListener('load', scheduleRender);
  window.addEventListener('resize', scheduleRealign);
  window.addEventListener('scroll', scheduleRealign, true);

  if (document.body) {
    observer.observe(document.body, { childList: true, subtree: true });
  }

  scheduleRender();
})();
