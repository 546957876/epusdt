"use strict";
(() => {
	let installMode = false;
	const isInstallRoute = () => window.location.pathname === "/install" || window.location.pathname.startsWith("/install/");
	const FIELD_CONTAINER_ID = "custom-initial-admin-fields";
	const USERNAME_ID = "custom-initial-admin-username";
	const PASSWORD_ID = "custom-initial-admin-password";
	const DB_TYPE_ID = "custom-db-type";
	const DB_SECTION_ID = "custom-db-config-fields";
	const DB_TEST_BUTTON_ID = "custom-db-test-button";
	const DB_ENSURE_BUTTON_ID = "custom-db-ensure-button";
	const DB_TEST_RESULT_ID = "custom-db-test-result";
	const state = {
		username: "admin",
		password: "",
		db_type: "sqlite",
		sqlite_database_filename: "",
		sqlite_table_prefix: "",
		mysql_host: "127.0.0.1",
		mysql_port: "3306",
		mysql_user: "gmpay",
		mysql_passwd: "",
		mysql_database: "gmpay",
		mysql_table_prefix: "",
		postgres_host: "127.0.0.1",
		postgres_port: "5432",
		postgres_user: "postgres",
		postgres_passwd: "",
		postgres_database: "gmpay",
		postgres_table_prefix: "",
	};

	const isInstallRequest = (url, method) => {
		try {
			const parsed = new URL(url, window.location.origin);
			return parsed.pathname === "/api/install" && String(method || "GET").toUpperCase() === "POST";
		} catch {
			return false;
		}
	};

	const installDefaultsAvailable = async () => {
		if (!isInstallRoute()) {
			return false;
		}
		try {
			const response = await fetch("/api/install/defaults", {
				method: "GET",
				cache: "no-store",
			});
			return response.ok;
		} catch {
			return false;
		}
	};

	const createFieldGroup = () => {
		const wrapper = document.createElement("div");
		wrapper.id = FIELD_CONTAINER_ID;
		wrapper.className = "grid gap-4";
		wrapper.innerHTML = `
			<div class="grid gap-4 sm:grid-cols-2">
				<label class="grid gap-2">
					<span class="font-medium text-sm">初始管理员账号 *</span>
					<input
						id="${USERNAME_ID}"
						class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
						placeholder="请输入管理员账号"
						autocomplete="username"
						spellcheck="false"
					>
					<span class="text-muted-foreground text-xs">首次登录后台时使用这个账号。</span>
				</label>
				<label class="grid gap-2">
					<span class="font-medium text-sm">初始管理员密码</span>
					<input
						id="${PASSWORD_ID}"
						class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
						placeholder="留空则自动生成"
						type="password"
						autocomplete="new-password"
					>
					<span class="text-muted-foreground text-xs">留空后由系统自动生成随机密码。</span>
				</label>
			</div>
			<div class="grid gap-2">
				<label class="grid gap-2">
					<span class="font-medium text-sm">主数据库 *</span>
					<select
						id="${DB_TYPE_ID}"
						class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
					>
						<option value="sqlite">SQLite</option>
						<option value="mysql">MySQL</option>
						<option value="postgres">PostgreSQL</option>
					</select>
				</label>
				<div id="${DB_SECTION_ID}" class="grid gap-4"></div>
				<div class="flex flex-col gap-2 sm:flex-row sm:items-center">
					<button id="${DB_TEST_BUTTON_ID}" type="button" class="inline-flex h-10 items-center justify-center rounded-md border border-input bg-background px-4 py-2 font-medium text-sm shadow-xs transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2">测试数据库连接</button>
					<button id="${DB_ENSURE_BUTTON_ID}" type="button" class="inline-flex h-10 items-center justify-center rounded-md border border-input bg-background px-4 py-2 font-medium text-sm shadow-xs transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2">自动创建数据库</button>
					<span id="${DB_TEST_RESULT_ID}" class="text-sm text-muted-foreground"></span>
				</div>
			</div>
		`;

		const usernameInput = wrapper.querySelector(`#${USERNAME_ID}`);
		const passwordInput = wrapper.querySelector(`#${PASSWORD_ID}`);
		if (usernameInput) {
			usernameInput.value = state.username;
			usernameInput.addEventListener("input", (event) => {
				state.username = event.target.value;
			});
		}
		if (passwordInput) {
			passwordInput.value = state.password;
			passwordInput.addEventListener("input", (event) => {
				state.password = event.target.value;
			});
		}

		const dbTypeSelect = wrapper.querySelector(`#${DB_TYPE_ID}`);
		const dbSection = wrapper.querySelector(`#${DB_SECTION_ID}`);
		const dbTestButton = wrapper.querySelector(`#${DB_TEST_BUTTON_ID}`);
		const dbEnsureButton = wrapper.querySelector(`#${DB_ENSURE_BUTTON_ID}`);
		const dbTestResult = wrapper.querySelector(`#${DB_TEST_RESULT_ID}`);
		const setDBTestResult = (message, variant = "muted") => {
			if (!dbTestResult) return;
			dbTestResult.textContent = message;
			dbTestResult.className = "text-sm";
			if (variant === "success") dbTestResult.classList.add("text-emerald-600", "dark:text-emerald-400");
			else if (variant === "error") dbTestResult.classList.add("text-red-600", "dark:text-red-400");
			else dbTestResult.classList.add("text-muted-foreground");
		};
		const buildDBPayload = () => ({
			initial_admin_username: (usernameInput?.value ?? state.username ?? "").trim(),
			initial_admin_password: (passwordInput?.value ?? state.password ?? "").trim(),
			db_type: state.db_type,
			sqlite_database_filename: state.sqlite_database_filename.trim(),
			sqlite_table_prefix: state.sqlite_table_prefix.trim(),
			mysql_host: state.mysql_host.trim(),
			mysql_port: state.mysql_port.trim(),
			mysql_user: state.mysql_user.trim(),
			mysql_passwd: state.mysql_passwd.trim(),
			mysql_database: state.mysql_database.trim(),
			mysql_table_prefix: state.mysql_table_prefix.trim(),
			postgres_host: state.postgres_host.trim(),
			postgres_port: state.postgres_port.trim(),
			postgres_user: state.postgres_user.trim(),
			postgres_passwd: state.postgres_passwd.trim(),
			postgres_database: state.postgres_database.trim(),
			postgres_table_prefix: state.postgres_table_prefix.trim(),
		});
		const runDBAction = async (url, pendingText, successText, extraPayload = {}) => {
			setDBTestResult(pendingText);
			const response = await fetch(url, {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ ...buildDBPayload(), ...extraPayload }),
			});
			const payload = await response.json().catch(() => ({}));
			if (!response.ok) throw new Error(payload?.error || "数据库操作失败");
			setDBTestResult(successText, "success");
		};
		const renderDbSection = () => {
			if (!dbSection) return;
			if (state.db_type === "sqlite") {
				dbSection.innerHTML = `
					<div class="grid gap-4 sm:grid-cols-2">
						<label class="grid gap-2">
							<span class="font-medium text-sm">SQLite 数据库文件名</span>
							<input data-db-key="sqlite_database_filename" class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2" placeholder="留空则使用 epusdt.db">
							<span class="text-muted-foreground text-xs">留空时使用默认的主库文件。</span>
						</label>
						<label class="grid gap-2">
							<span class="font-medium text-sm">SQLite 表前缀</span>
							<input data-db-key="sqlite_table_prefix" class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2" placeholder="可选">
						</label>
					</div>`;
			} else if (state.db_type === "mysql") {
				dbSection.innerHTML = `
					<div class="grid gap-4 sm:grid-cols-2">
						<label class="grid gap-2"><span class="font-medium text-sm">MySQL 地址 *</span><input data-db-key="mysql_host" class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2" placeholder="127.0.0.1"></label>
						<label class="grid gap-2"><span class="font-medium text-sm">MySQL 端口 *</span><input data-db-key="mysql_port" class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2" placeholder="3306"></label>
						<label class="grid gap-2"><span class="font-medium text-sm">MySQL 用户名 *</span><input data-db-key="mysql_user" class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2" placeholder="gmpay"></label>
						<label class="grid gap-2"><span class="font-medium text-sm">MySQL 密码</span><input data-db-key="mysql_passwd" type="password" class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2" placeholder="可选，重装时留空保留旧密码"></label>
						<label class="grid gap-2"><span class="font-medium text-sm">MySQL 数据库名 *</span><input data-db-key="mysql_database" class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2" placeholder="gmpay"></label>
						<label class="grid gap-2"><span class="font-medium text-sm">MySQL 表前缀</span><input data-db-key="mysql_table_prefix" class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2" placeholder="可选"></label>
					</div>`;
			} else {
				dbSection.innerHTML = `
					<div class="grid gap-4 sm:grid-cols-2">
						<label class="grid gap-2"><span class="font-medium text-sm">PostgreSQL 地址 *</span><input data-db-key="postgres_host" class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2" placeholder="127.0.0.1"></label>
						<label class="grid gap-2"><span class="font-medium text-sm">PostgreSQL 端口 *</span><input data-db-key="postgres_port" class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2" placeholder="5432"></label>
						<label class="grid gap-2"><span class="font-medium text-sm">PostgreSQL 用户名 *</span><input data-db-key="postgres_user" class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2" placeholder="postgres"></label>
						<label class="grid gap-2"><span class="font-medium text-sm">PostgreSQL 密码</span><input data-db-key="postgres_passwd" type="password" class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2" placeholder="可选，重装时留空保留旧密码"></label>
						<label class="grid gap-2"><span class="font-medium text-sm">PostgreSQL 数据库名 *</span><input data-db-key="postgres_database" class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2" placeholder="gmpay"></label>
						<label class="grid gap-2"><span class="font-medium text-sm">PostgreSQL 表前缀</span><input data-db-key="postgres_table_prefix" class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2" placeholder="可选"></label>
					</div>`;
			}
			dbSection.querySelectorAll("[data-db-key]").forEach((input) => {
				const key = input.getAttribute("data-db-key");
				input.value = state[key] ?? "";
				input.addEventListener("input", (event) => {
					state[key] = event.target.value;
				});
			});
		};

		if (dbTypeSelect) {
			dbTypeSelect.value = state.db_type;
			dbTypeSelect.addEventListener("change", (event) => {
				state.db_type = event.target.value;
				setDBTestResult("");
				renderDbSection();
			});
		}
		if (dbTestButton) {
			dbTestButton.addEventListener("click", async () => {
				dbTestButton.disabled = true;
				if (dbEnsureButton) dbEnsureButton.disabled = true;
				try {
					await runDBAction("/api/install/test-db", "正在测试数据库连接...", "数据库连接成功");
				} catch {
					setDBTestResult("数据库连接失败", "error");
				} finally {
					dbTestButton.disabled = false;
					if (dbEnsureButton) dbEnsureButton.disabled = false;
				}
			});
		}
		if (dbEnsureButton) {
			dbEnsureButton.addEventListener("click", async () => {
				dbEnsureButton.disabled = true;
				if (dbTestButton) dbTestButton.disabled = true;
				try {
					await runDBAction("/api/install/ensure-db", "正在创建数据库...", "数据库已创建并连接成功", { create_database_if_missing: true });
				} catch (error) {
					setDBTestResult(error?.message || "数据库创建失败", "error");
				} finally {
					dbEnsureButton.disabled = false;
					if (dbTestButton) dbTestButton.disabled = false;
				}
			});
		}
		renderDbSection();
		return wrapper;
	};

	const waitForServiceReady = async () => {
		const deadline = Date.now() + 20000;
		let lastError = "服务启动超时";
		while (Date.now() < deadline) {
			try {
				const response = await fetch("/admin/api/v1/auth/init-password-hash", { method: "GET", cache: "no-store" });
				const payload = await response.json().catch(() => ({}));
				if (response.ok) return true;
				lastError = payload?.message || payload?.error || lastError;
			} catch (error) {
				lastError = error?.message || lastError;
			}
			await new Promise((resolve) => setTimeout(resolve, 800));
		}
		throw new Error(lastError);
	};

	const mountInstallFields = () => {
		if (!installMode || !isInstallRoute()) {
			const existing = document.getElementById(FIELD_CONTAINER_ID);
			if (existing) existing.remove();
			return;
		}
		const form = document.querySelector("#app form");
		if (!form) return;
		if (document.getElementById(FIELD_CONTAINER_ID)) return;
		const submitButton = form.querySelector('button[type="submit"]');
		const fieldGroup = createFieldGroup();
		if (submitButton?.parentNode) submitButton.parentNode.insertBefore(fieldGroup, submitButton);
		else form.appendChild(fieldGroup);
	};

	const observer = new MutationObserver(() => mountInstallFields());
	observer.observe(document.documentElement, { childList: true, subtree: true });

	const installStatusMessages = {
		idle: "",
		preparing: "正在准备安装环境...",
		starting: "配置已保存，正在等待服务启动...",
	};
	let installStatusNode = null;
	const ensureInstallStatusNode = () => {
		if (installStatusNode && document.body.contains(installStatusNode)) return installStatusNode;
		const form = document.querySelector("#app form");
		if (!form) return null;
		installStatusNode = document.createElement("div");
		installStatusNode.className = "text-sm text-muted-foreground";
		installStatusNode.id = "custom-install-status";
		form.appendChild(installStatusNode);
		return installStatusNode;
	};
	const setInstallStatus = (keyOrMessage, variant = "muted") => {
		const node = ensureInstallStatusNode();
		if (!node) return;
		node.textContent = installStatusMessages[keyOrMessage] || keyOrMessage || "";
		node.className = "text-sm";
		if (variant === "success") node.classList.add("text-emerald-600", "dark:text-emerald-400");
		else if (variant === "error") node.classList.add("text-red-600", "dark:text-red-400");
		else node.classList.add("text-muted-foreground");
	};

	if (!window.__epusdtInstallXHRPatched) {
		window.__epusdtInstallXHRPatched = true;
		const originalFetch = window.fetch.bind(window);
		window.fetch = async (input, init) => {
			const requestUrl = typeof input === "string" ? input : input?.url;
			const requestMethod = init?.method || (typeof input === "string" ? "GET" : input?.method) || "GET";
			if (!installMode || !isInstallRequest(requestUrl, requestMethod)) return originalFetch(input, init);
			setInstallStatus("preparing");
			const response = await originalFetch(input, init);
			if (!response.ok) return response;
			setInstallStatus("starting");
			try {
				await waitForServiceReady();
				setInstallStatus("服务启动成功", "success");
			} catch (error) {
				setInstallStatus(error?.message || "服务启动失败，请查看终端日志", "error");
			}
			return response;
		};
	}

	const refreshInstallMode = async () => {
		installMode = await installDefaultsAvailable();
		mountInstallFields();
	};
	const hookHistory = () => {
		const wrap = (method) => {
			const original = history[method];
			if (typeof original !== "function") return;
			history[method] = function (...args) {
				const result = original.apply(this, args);
				queueMicrotask(() => { refreshInstallMode().catch(() => void 0); });
				return result;
			};
		};
		wrap("pushState");
		wrap("replaceState");
		window.addEventListener("popstate", () => { refreshInstallMode().catch(() => void 0); });
	};
	hookHistory();
	if (document.readyState === "loading") {
		document.addEventListener("DOMContentLoaded", () => { refreshInstallMode().catch(() => void 0); }, { once: true });
	} else {
		refreshInstallMode().catch(() => void 0);
	}
})();
