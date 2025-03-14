# Copyright 2025 Greptime Team
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

## Tool Versions
KAWKEYE_VERSION ?= v6.0.0

.PHONY: hawkeye
hawkeye: ## Install hawkeye.
	curl --proto '=https' --tlsv1.2 -LsSf https://github.com/korandoru/hawkeye/releases/download/${KAWKEYE_VERSION}/hawkeye-installer.sh | sh

.PHONY: check-license
check-license: ## Check License Header.
	hawkeye check --config licenserc.toml

.PHONY: format-license
format-license: ## Format License Header.
	hawkeye format --config licenserc.toml

.PHONY: remove-license
remove-license: ## Remove License Header.
	hawkeye remove --config licenserc.toml
