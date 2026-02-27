#!/usr/bin/env python3
with open('web/index.html', 'r') as f:
    content = f.read()

marker = '    <!-- Login Modal (standard) -->'

setup_html = """    <!-- Setup Wizard (first run) -->
    <div class="setup-overlay" id="setupOverlay">
        <div class="setup-card">
            <div class="setup-logo">CineVault Setup</div>
            <div class="setup-subtitle">Create your admin account to get started</div>
            <div id="setupMessage"></div>
            <form id="setupForm">
                <div class="form-group">
                    <label>Username</label>
                    <input type="text" id="setupUsername" required autocomplete="username">
                </div>
                <div class="setup-row">
                    <div class="form-group">
                        <label>First Name</label>
                        <input type="text" id="setupFirstName" required>
                    </div>
                    <div class="form-group">
                        <label>Last Name</label>
                        <input type="text" id="setupLastName" required>
                    </div>
                </div>
                <div class="form-group">
                    <label>Email</label>
                    <input type="email" id="setupEmail" required autocomplete="email">
                </div>
                <div class="form-group">
                    <label>Password</label>
                    <input type="password" id="setupPassword" required autocomplete="new-password">
                </div>
                <div class="form-group">
                    <label>PIN (for fast login)</label>
                    <input type="text" id="setupPin" inputmode="numeric" maxlength="10" autocomplete="off" placeholder="4-digit PIN">
                    <div class="setup-pin-note">Numeric only. Used for quick sign-in on shared devices.</div>
                </div>
                <button type="submit" class="btn-primary" style="width:100%;margin-top:16px;">Create Admin Account</button>
            </form>
        </div>
    </div>

"""

content = content.replace(marker, setup_html + marker, 1)

with open('web/index.html', 'w') as f:
    f.write(content)

print('Setup wizard HTML inserted successfully')
