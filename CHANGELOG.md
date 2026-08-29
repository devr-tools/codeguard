# Changelog

## [1.7.2](https://github.com/devr-tools/codeguard/compare/v1.7.1...v1.7.2) (2026-08-29)


### Bug Fixes

* trigger release for scanner precision fixes ([caaa85e](https://github.com/devr-tools/codeguard/commit/caaa85e5c35765a54c905966dcf300ff2ed39487))
* trigger release for scanner precision fixes ([#161](https://github.com/devr-tools/codeguard/issues/161)) ([4cbb945](https://github.com/devr-tools/codeguard/commit/4cbb9458baf773a666ab0efc682850b52daf9624))

## [1.7.1](https://github.com/devr-tools/codeguard/compare/v1.7.0...v1.7.1) (2026-08-25)


### Bug Fixes

* gate Rust dead-code Cargo execution ([0440f94](https://github.com/devr-tools/codeguard/commit/0440f94ba0bb782abd5d1bf6aad40b2c84a797cd))
* reduce LMP false positives ([12c2255](https://github.com/devr-tools/codeguard/commit/12c2255dfa738b61ef59f2364b8607b34d395b18))
* reduce LMP false positives ([#158](https://github.com/devr-tools/codeguard/issues/158)) ([747086a](https://github.com/devr-tools/codeguard/commit/747086acc6ca52279cd0fbc93767d72c9319ecf5))
* support Go 1.23 in Rust dead-code test ([427c6f6](https://github.com/devr-tools/codeguard/commit/427c6f60e51cfcaf160bd6e555af4773a8d4d07d))

## [1.7.0](https://github.com/devr-tools/codeguard/compare/v1.6.0...v1.7.0) (2026-08-24)


### Features

* add toolchain dead-code scans ([03bd363](https://github.com/devr-tools/codeguard/commit/03bd3633d8e13a063d2234b257bdd8bcc0a63db4))


### Bug Fixes

* reduce LMP false positives ([995501e](https://github.com/devr-tools/codeguard/commit/995501e8f7c3b7e7c51ad701db41b3a53dcd1419))
* satisfy dead-code lint gates ([67ed8ae](https://github.com/devr-tools/codeguard/commit/67ed8aeec61109082e92302a929d4823406c1c7e))

## [1.6.0](https://github.com/devr-tools/codeguard/compare/v1.5.4...v1.6.0) (2026-08-22)


### Features

* **benchmark:** normalize external scanner reports ([31cb15a](https://github.com/devr-tools/codeguard/commit/31cb15ae84b9316744b85fe396dfc4809805314e))


### Bug Fixes

* **benchmark:** release provenance and improve benchmark/security coverage ([#152](https://github.com/devr-tools/codeguard/issues/152)) ([81b7cd9](https://github.com/devr-tools/codeguard/commit/81b7cd9989e8c73a13f1f8bff955e1d5b74eb650))
* **benchmark:** satisfy strict lint ([ff61aed](https://github.com/devr-tools/codeguard/commit/ff61aedf75a4964e3c30142c6a735e6dfa35b4de))
* **ci:** satisfy codeguard self-scan ([a3bf7f6](https://github.com/devr-tools/codeguard/commit/a3bf7f6da0d9ce994487bffa87bd6f433080e615))
* **ci:** use tagged SLSA generator workflow ([e0b9a95](https://github.com/devr-tools/codeguard/commit/e0b9a954bd0940bd0e8bbb86c0604c0ff41a10bd))
* **security:** reduce secret and OWASP detector gaps ([7e9e3f5](https://github.com/devr-tools/codeguard/commit/7e9e3f5b17e33278e9923f6e9c26d18a58bd43c4))

## [1.5.4](https://github.com/devr-tools/codeguard/compare/v1.5.3...v1.5.4) (2026-08-19)


### Bug Fixes

* bound waiver audit history reads ([4ecb4da](https://github.com/devr-tools/codeguard/commit/4ecb4da9ac5ea6e22aa67acd4f0f37bdda869dbd))
* cap rule stats history reads ([8aa4345](https://github.com/devr-tools/codeguard/commit/8aa4345da8c3cd16bf61908c89bf19830150e4fe))
* detect zero HTTP client timeouts ([c664c33](https://github.com/devr-tools/codeguard/commit/c664c338c2806a70bfbea5bdbb7c5a1409120396))
* **parser:** make Python call extraction linear ([e3dd6d4](https://github.com/devr-tools/codeguard/commit/e3dd6d44f132548a2b43874de2a49ed4f5936f82))
* **parser:** make Python call extraction linear ([#140](https://github.com/devr-tools/codeguard/issues/140)) ([0970150](https://github.com/devr-tools/codeguard/commit/0970150a8240b8c45645209de208aa8a9e4ac0c2))
* **quality:** bound AI provenance git output ([#139](https://github.com/devr-tools/codeguard/issues/139)) ([dc17db9](https://github.com/devr-tools/codeguard/commit/dc17db9d1e1d8fd0017c29515716b4e1b5b73e87))
* **quality:** cap git provenance output ([8e95804](https://github.com/devr-tools/codeguard/commit/8e95804bbabcb3e374b36371c521480e9da96e0e))
* reject unsafe typeof null guards ([e2f552e](https://github.com/devr-tools/codeguard/commit/e2f552e9e4a5e943e707c3f7ebfc01b08c2cc1aa))
* **reliability:** bound zero timeout detection ([e838116](https://github.com/devr-tools/codeguard/commit/e838116655194a7302ad1d559a385db82f3cbc53))
* scope nullable block guards to their bodies ([fcb3023](https://github.com/devr-tools/codeguard/commit/fcb3023f9766f99c5712dbdbc1f35222d1965c98))
* **security:** avoid retaining dynamic regexes ([1a55a38](https://github.com/devr-tools/codeguard/commit/1a55a389fca45fbbd302974c39bcd107b058955f))
* **security:** avoid retaining dynamic regexes ([#145](https://github.com/devr-tools/codeguard/issues/145)) ([3b6aa51](https://github.com/devr-tools/codeguard/commit/3b6aa513c37911f26032df7704d12adeef6fe331))
* **security:** bound git history secret scans ([3fa7c09](https://github.com/devr-tools/codeguard/commit/3fa7c093a3ad8501ec860c035eeb527aa9de73fb))
* **security:** bound git-history parsing to prevent DoS ([#146](https://github.com/devr-tools/codeguard/issues/146)) ([d0eb42f](https://github.com/devr-tools/codeguard/commit/d0eb42f06625f360b3322028019a0334c40ad890))
* use errors.Is for bounded history reads ([b5076ff](https://github.com/devr-tools/codeguard/commit/b5076ffe675f9073424a3cba57dbc7bee95a474a))

## [1.5.3](https://github.com/devr-tools/codeguard/compare/v1.5.2...v1.5.3) (2026-08-19)


### Bug Fixes

* **ci:** require immutable GitHub Action refs ([#111](https://github.com/devr-tools/codeguard/issues/111)) ([cbcea34](https://github.com/devr-tools/codeguard/commit/cbcea34bc77d0aa79bcf2fc6ddfd60108e201fcf))
* **ci:** validate Homebrew formula version ([c6b52bd](https://github.com/devr-tools/codeguard/commit/c6b52bd432d3233203170f18839e9bb31c673b68))
* **ci:** validate Homebrew formula version ([#135](https://github.com/devr-tools/codeguard/issues/135)) ([3b14798](https://github.com/devr-tools/codeguard/commit/3b14798d838966cc6863bef48debfd88ff4d2ecf))
* **config:** contain performance history path ([#126](https://github.com/devr-tools/codeguard/issues/126)) ([5f2f133](https://github.com/devr-tools/codeguard/commit/5f2f133bcbece7418ee59d4c841cd3d8764a16b4))
* make goroutine loop detection linear ([b560584](https://github.com/devr-tools/codeguard/commit/b5605841149067b58e1d7271c45f45e304dd39ee))
* **parser:** bound tree-sitter scan resources ([c83a2ce](https://github.com/devr-tools/codeguard/commit/c83a2cebfce9f276ba9eb3d05171d93d80f8fa67))
* **parser:** bound tree-sitter scan resources ([#133](https://github.com/devr-tools/codeguard/issues/133)) ([a0e4e3c](https://github.com/devr-tools/codeguard/commit/a0e4e3cd61c4d62fddd4ce22e1f224e4730b553a))
* **performance:** cap complexity regression reads ([fca8c4e](https://github.com/devr-tools/codeguard/commit/fca8c4edfc37b360dc84382af2cbc5fc2b33cf26))
* **performance:** cap complexity-regression reads to use corpus reader ([#131](https://github.com/devr-tools/codeguard/issues/131)) ([25c173c](https://github.com/devr-tools/codeguard/commit/25c173c86c2fae69973712be6d662ffb9f248069))
* prevent tsconfig from narrowing security scans ([a464824](https://github.com/devr-tools/codeguard/commit/a464824535aedf942bdd646a548033f62d55e790))
* **quality:** skip rolling-hash precompute when no full clone window exists ([#125](https://github.com/devr-tools/codeguard/issues/125)) ([19c1874](https://github.com/devr-tools/codeguard/commit/19c187477be85aae533b272a6643e4fbe84bdac6))
* **release:** protect trusted publishing environment ([4350eb9](https://github.com/devr-tools/codeguard/commit/4350eb91ac2d4406acc71930a3142fe8b0e9092a))
* **release:** require protected `release-publish` environment for npm/PyPI OIDC publishing ([#132](https://github.com/devr-tools/codeguard/issues/132)) ([43cff7e](https://github.com/devr-tools/codeguard/commit/43cff7e4f957406c6c288105e290c70debd75a90))
* **reliability:** make goroutine-in-loop detection linear to avoid quadratic AST walks ([#130](https://github.com/devr-tools/codeguard/issues/130)) ([23f7558](https://github.com/devr-tools/codeguard/commit/23f7558e1858c53e12a6209e8726331b8a8b48c0))
* **security:** contain slop history writes ([d5209d5](https://github.com/devr-tools/codeguard/commit/d5209d5fb745118e045f7d912edea95265e8a0a5))
* **security:** contain slop history writes ([#136](https://github.com/devr-tools/codeguard/issues/136)) ([6ebbafc](https://github.com/devr-tools/codeguard/commit/6ebbafce9529005a230aa17751f9b259baa9a6b7))
* **security:** include targeted node_modules files in TypeScript semantic scans ([#137](https://github.com/devr-tools/codeguard/issues/137)) ([0ef6ab1](https://github.com/devr-tools/codeguard/commit/0ef6ab1305727e217b4243c3071690e5d3e3cb04))
* **security:** scan targeted node modules semantically ([728e527](https://github.com/devr-tools/codeguard/commit/728e527810d22d8b382430d80888eeac7b26cd31))

## [1.5.2](https://github.com/devr-tools/codeguard/compare/v1.5.1...v1.5.2) (2026-08-18)


### Bug Fixes

* **ai:** reject wrapped triage verdicts ([c1b24a5](https://github.com/devr-tools/codeguard/commit/c1b24a5792b556fce9a73f348ca9c53c302025e5))
* **ai:** reject wrapped triage verdicts ([#113](https://github.com/devr-tools/codeguard/issues/113)) ([170844d](https://github.com/devr-tools/codeguard/commit/170844db8cb263181c9ac9842c448b42433f99d6))
* **ai:** scale triage request timeout ([a8a6308](https://github.com/devr-tools/codeguard/commit/a8a6308fd07ee6b11bf124fd54d3c8b19f244e10))
* **ai:** scale triage timeout and report build versions ([#101](https://github.com/devr-tools/codeguard/issues/101)) ([8434c22](https://github.com/devr-tools/codeguard/commit/8434c22b4365f1def03c718f52d58d6b4a088b30))
* bound one-use abstraction analysis ([328f345](https://github.com/devr-tools/codeguard/commit/328f3455d2328e8b8539dc051a3de802a2ab138a))
* **ci:** restore Homebrew version extraction ([ed71e23](https://github.com/devr-tools/codeguard/commit/ed71e235bfe5a53b16787bc338cbc89e4bb1555a))
* **config:** contain legibility history path ([94cb937](https://github.com/devr-tools/codeguard/commit/94cb937a2a5ce039ab5e7a172e6a0987e707ca6e))
* **config:** resolve dangling symlinks in artifact paths ([fb5734d](https://github.com/devr-tools/codeguard/commit/fb5734d009e7764094e43f2f2e706ac78f56647e))
* **config:** restore complete config loader ([7ca5d02](https://github.com/devr-tools/codeguard/commit/7ca5d0211acaac4f69a7e0a11ee34f9d24cf62e5))
* **config:** validate derived legibility-history path ([#106](https://github.com/devr-tools/codeguard/issues/106)) ([59b0ba7](https://github.com/devr-tools/codeguard/commit/59b0ba7b7fcb4905c2b4dd38e62ee6b03af4b3cc))
* **cpp:** gate compiler validation behind trust opt-in ([aee079d](https://github.com/devr-tools/codeguard/commit/aee079d151d7af662154f18a3e54162708bfd830))
* **cpp:** gate compiler validation behind trust opt-in ([#105](https://github.com/devr-tools/codeguard/issues/105)) ([1970289](https://github.com/devr-tools/codeguard/commit/1970289207597f2682ae3556d3ec006aba6a37fa))
* **diff:** merge colliding diff scopes for overlapping targets ([#115](https://github.com/devr-tools/codeguard/issues/115)) ([90744cb](https://github.com/devr-tools/codeguard/commit/90744cb2e06a34351b07f6b9f0dc1bb3a0a26c41))
* **diff:** merge scopes for overlapping targets ([1ca3fe1](https://github.com/devr-tools/codeguard/commit/1ca3fe1827e105c06aa2995511a1b1dad5a8a858))
* fail closed when default scan config is missing ([07bb894](https://github.com/devr-tools/codeguard/commit/07bb8949145322dd0c407a4bc9dc0e2fc955c538))
* make C-like class smell scans linear ([fabf71d](https://github.com/devr-tools/codeguard/commit/fabf71d82869dada6da9e99d5a5b1a10f57c3c3a))
* **performance:** bound framework regex matches ([62915df](https://github.com/devr-tools/codeguard/commit/62915df68a52c7276584ec31cf1fda2cc322d77b))
* **performance:** bound framework regex matches ([#128](https://github.com/devr-tools/codeguard/issues/128)) ([52cfb39](https://github.com/devr-tools/codeguard/commit/52cfb39860dd71141dedc2aa1a6e49d10fa420ee))
* prevent filename injection in format targets ([e0b7ce8](https://github.com/devr-tools/codeguard/commit/e0b7ce8ed64490292726d220836272b400f67ff9))
* prevent SARIF narrative secret leakage ([aebc32c](https://github.com/devr-tools/codeguard/commit/aebc32c78f66391c67fa3c8833cdd88a49ce574d))
* **quality:** bound pnpm lockfile scanning ([551993b](https://github.com/devr-tools/codeguard/commit/551993b97a075aa6d76f9f4ff92370b1054a94bc))
* **quality:** bound pnpm lockfile scanning ([#121](https://github.com/devr-tools/codeguard/issues/121)) ([d44cd26](https://github.com/devr-tools/codeguard/commit/d44cd264a6a582ce2d00c3952a9fa33379980995))
* **quality:** preserve capped reader failures ([5a508f6](https://github.com/devr-tools/codeguard/commit/5a508f6b238b3e5d98009c6ab7e34fe4e13995d3))
* **quality:** preserve capped reader failures to prevent uncapped reads in diff scans ([#120](https://github.com/devr-tools/codeguard/issues/120)) ([2e82480](https://github.com/devr-tools/codeguard/commit/2e824802c618b48c6980187cfe9acf1f019a1664))
* **quality:** reuse duplicated knowledge regex ([9f112fd](https://github.com/devr-tools/codeguard/commit/9f112fd3807c0af4fe0808d3cd98377e0c86eb3a))
* respect TypeScript config root exclusions ([e86b29b](https://github.com/devr-tools/codeguard/commit/e86b29b3c4a5bf9f1690a3c1099e6dc513845045))
* **security:** bound duplicate-code detection work ([2ea8833](https://github.com/devr-tools/codeguard/commit/2ea883318d0720a53415c14f3f36a691a001bbdb))
* **security:** bound duplicate-code detection work ([#112](https://github.com/devr-tools/codeguard/issues/112)) ([cc6b95c](https://github.com/devr-tools/codeguard/commit/cc6b95cb287f539cbc597993028251385e5d77c7))
* **security:** detect multiline C++ TLS disabling ([bc382c2](https://github.com/devr-tools/codeguard/commit/bc382c23dc1a2371fe391f2a56ef2b02f2072013))
* **security:** gate benchmark execution on trust opt-in ([8dacc79](https://github.com/devr-tools/codeguard/commit/8dacc791f3952455d0fd4d1b50fe4b4383918d43))
* **security:** gate benchmark execution on trust opt-in ([#102](https://github.com/devr-tools/codeguard/issues/102)) ([a2ea743](https://github.com/devr-tools/codeguard/commit/a2ea7439593ccfc859f3549f416e18ba386a31ec))
* **security:** track identifier lines linearly ([82ade0f](https://github.com/devr-tools/codeguard/commit/82ade0f973293c930973f2c2f5cb10d0ccaaab82))
* **security:** track identifier lines linearly ([#110](https://github.com/devr-tools/codeguard/issues/110)) ([dc348e8](https://github.com/devr-tools/codeguard/commit/dc348e8e8b1bc61d520d42dc9a8ce4aeaa85c850))
* **semantic:** contain source snapshots within target ([5a24381](https://github.com/devr-tools/codeguard/commit/5a2438184c8a96f7fe3b2e5c266abf94b4536953))
* **version:** read module build info ([2f33945](https://github.com/devr-tools/codeguard/commit/2f33945005505e93e9045357ba2006f1a1fa584e))

## [1.5.1](https://github.com/devr-tools/codeguard/compare/v1.5.0...v1.5.1) (2026-08-18)


### Bug Fixes

* **ai:** accept fenced JSON in triage verdicts ([93325b6](https://github.com/devr-tools/codeguard/commit/93325b6063e4eee0a13f6eb62e4e86e1aadb8cfa))
* **ai:** accept fenced JSON in triage verdicts ([#98](https://github.com/devr-tools/codeguard/issues/98)) ([8d8e31e](https://github.com/devr-tools/codeguard/commit/8d8e31e442f0a05cbf117de3161b0d8943886e16))

## [1.5.0](https://github.com/devr-tools/codeguard/compare/v1.4.1...v1.5.0) (2026-08-16)


### Features

* add waiver audit cleanup command ([a1895f7](https://github.com/devr-tools/codeguard/commit/a1895f74c5a1eab87887adafed171beb8c6435e6))

## [1.4.1](https://github.com/devr-tools/codeguard/compare/v1.4.0...v1.4.1) (2026-08-11)


### Bug Fixes

* allow configless folder scans ([6d6e6ca](https://github.com/devr-tools/codeguard/commit/6d6e6cacdaf276381aed09e940aa1e8af7325e3b))
* allow configless repo scans ([5405103](https://github.com/devr-tools/codeguard/commit/5405103ed53cf17dcd9b29fc5c9e1c908095cede))

## [1.4.0](https://github.com/devr-tools/codeguard/compare/v1.3.9...v1.4.0) (2026-08-11)


### Features

* support folder scans ([721026d](https://github.com/devr-tools/codeguard/commit/721026d40d6e14684187dc5c2fed59664a32151d))

## [1.3.9](https://github.com/devr-tools/codeguard/compare/v1.3.8...v1.3.9) (2026-07-29)


### Bug Fixes

* keep domain structural smell positives ([a8b5a62](https://github.com/devr-tools/codeguard/commit/a8b5a62e50bd543f2a684b9391767d18f8749710))
* retune precision for monorepo false positives ([7422df9](https://github.com/devr-tools/codeguard/commit/7422df9d20b60d6f796e50bf7a77407994275f32))
* satisfy precision retune lint checks ([d55e5ec](https://github.com/devr-tools/codeguard/commit/d55e5ec32f835df4b624e5bcab3e65ba661e5d9b))

## [1.3.8](https://github.com/devr-tools/codeguard/compare/v1.3.7...v1.3.8) (2026-07-28)


### Bug Fixes

* credit typescript null narrowing ([be628a4](https://github.com/devr-tools/codeguard/commit/be628a4296d9031e6e5941a128dd599470633f5c))
* focus integer overflow on allocation risk ([712ac16](https://github.com/devr-tools/codeguard/commit/712ac162b66984e002f9f558c4065eab7e95ae6c))
* make boolean naming precision contract based ([b1da7ca](https://github.com/devr-tools/codeguard/commit/b1da7ca565ce2a29b48c76cb02544ad49a30a44d))
* narrow boundary input detection ([31c1bc4](https://github.com/devr-tools/codeguard/commit/31c1bc4481023b9a2cd221efcae2a4a3c2655573))
* remove unused boolean expression parameter ([8f0b1af](https://github.com/devr-tools/codeguard/commit/8f0b1af4d48f9bfa812c300cfce49fc188d8c7c5))
* retune naming and return precision ([43813e4](https://github.com/devr-tools/codeguard/commit/43813e45f378e33a8c0777893fa74ac4218820cc))
* retune quality precision false positives ([c61b831](https://github.com/devr-tools/codeguard/commit/c61b831ce4e3d97e0f001b18f67a4d492b2bd5cc))
* skip validation throws in control-flow rule ([280ccdd](https://github.com/devr-tools/codeguard/commit/280ccdd0546a5035ae9a2573ced291d1c107956c))

## [1.3.7](https://github.com/devr-tools/codeguard/compare/v1.3.6...v1.3.7) (2026-07-28)


### Bug Fixes

* narrow defensive precision broadening ([3a237f0](https://github.com/devr-tools/codeguard/commit/3a237f0f3973c12ed9c19a3d194c53bea46661a8))
* reduce defensive false positives ([81388fd](https://github.com/devr-tools/codeguard/commit/81388fdda06290889dac0e06c61ba6b8fb223aa4))
* reduce defensive precision false positives ([#85](https://github.com/devr-tools/codeguard/issues/85)) ([4ce5e1d](https://github.com/devr-tools/codeguard/commit/4ce5e1d96f39a90a3a6cda44158d31e046b6be0a))

## [1.3.6](https://github.com/devr-tools/codeguard/compare/v1.3.5...v1.3.6) (2026-07-28)


### Bug Fixes

* report retry-mitigated sequence debt ([8c7a6d5](https://github.com/devr-tools/codeguard/commit/8c7a6d54a190a6ed2c7c899088466e28ed58422a))
* retune defensive precision followups ([a9367ae](https://github.com/devr-tools/codeguard/commit/a9367ae38197ee4d1a23fdfaeafa56267a8d2cbf))
* retune defensive precision followups ([#83](https://github.com/devr-tools/codeguard/issues/83)) ([ef6111a](https://github.com/devr-tools/codeguard/commit/ef6111a29c2102a32789e41870b625fbe69ca8a4))
* treat allocators as command style ([4d7dfa9](https://github.com/devr-tools/codeguard/commit/4d7dfa955d381f7a680e2a293ee7123c1484fca4))

## [1.3.5](https://github.com/devr-tools/codeguard/compare/v1.3.4...v1.3.5) (2026-07-28)


### Bug Fixes

* cover remaining precision retunes ([1fd6f2b](https://github.com/devr-tools/codeguard/commit/1fd6f2b55a0e6a1f68b9dbfb475d3c55ee0d3277))
* retune precision false positives ([6a09863](https://github.com/devr-tools/codeguard/commit/6a09863f48212492e7ce87f4d1f70f260499988b))
* retune precision false positives ([#81](https://github.com/devr-tools/codeguard/issues/81)) ([a63ff23](https://github.com/devr-tools/codeguard/commit/a63ff236d7ca77a14b15a369241fb14583cecb2c))
* split defensive sequence collision signal ([c734e9a](https://github.com/devr-tools/codeguard/commit/c734e9ab4e26bb6122a88835aab57972237d1c27))

## [1.3.4](https://github.com/devr-tools/codeguard/compare/v1.3.3...v1.3.4) (2026-07-28)


### Bug Fixes

* reduce precision false positives ([7c52ee1](https://github.com/devr-tools/codeguard/commit/7c52ee115fc0d78bc4ffa38abb93962bcf08bed0))
* reduce precision false positives ([#79](https://github.com/devr-tools/codeguard/issues/79)) ([7a42827](https://github.com/devr-tools/codeguard/commit/7a42827d6548dbf5d82778b027400338b796463d))

## [1.3.3](https://github.com/devr-tools/codeguard/compare/v1.3.2...v1.3.3) (2026-07-28)


### Bug Fixes

* reduce high-volume precision false positives ([cb19b29](https://github.com/devr-tools/codeguard/commit/cb19b295fcc2de870f660be0f98ce62c39e3945d))
* reduce high-volume precision false positives ([#77](https://github.com/devr-tools/codeguard/issues/77)) ([4cdd646](https://github.com/devr-tools/codeguard/commit/4cdd64697e3f605c9e6fb244e1d9a78d84296acf))

## [1.3.2](https://github.com/devr-tools/codeguard/compare/v1.3.1...v1.3.2) (2026-07-28)


### Bug Fixes

* add next and nest framework precision coverage ([220826e](https://github.com/devr-tools/codeguard/commit/220826eb2d9fea116013fb531bdeb88596a93ce5))
* harden react false positive handling ([2fde7bb](https://github.com/devr-tools/codeguard/commit/2fde7bb41171bf79f7835b462f4ae61cbec00a04))
* harden React false positive handling ([#75](https://github.com/devr-tools/codeguard/issues/75)) ([752bd41](https://github.com/devr-tools/codeguard/commit/752bd4118b2a7f83924fda016d0651fdbe6da632))

## [1.3.1](https://github.com/devr-tools/codeguard/compare/v1.3.0...v1.3.1) (2026-07-28)


### Bug Fixes

* harden hidden mutation precision ([8f77a74](https://github.com/devr-tools/codeguard/commit/8f77a747d186e43cecfaecd7e9a29f84a57d5978))
* harden UI false positive precision ([b46dfe7](https://github.com/devr-tools/codeguard/commit/b46dfe77d5fe308e45e8734375b1a6e0bf983bb2))
* react hook hidden mutation noise ([#73](https://github.com/devr-tools/codeguard/issues/73)) ([3fa1b1c](https://github.com/devr-tools/codeguard/commit/3fa1b1cf9a06994edf18ed72429d5047bafd2c44))
* reduce hidden mutation noise for React hooks ([c4f61c9](https://github.com/devr-tools/codeguard/commit/c4f61c961f633449daf1ebb251c54058508b1d4b))
* satisfy lint for false positive hardening ([62643da](https://github.com/devr-tools/codeguard/commit/62643da4d57a6eb7fde0926de0121333e605902f))

## [1.3.0](https://github.com/devr-tools/codeguard/compare/v1.2.2...v1.3.0) (2026-07-27)


### Features

* add abstraction quality checks ([fc5a7ec](https://github.com/devr-tools/codeguard/commit/fc5a7ec06b44aa7e4ddb2c637eaf809ca4c7cc00))
* add change safety diff detectors ([ebc2f80](https://github.com/devr-tools/codeguard/commit/ebc2f801fa180d075f01f3b0939155b70de0d4a8))
* add change safety PR summary metrics ([5b10ecd](https://github.com/devr-tools/codeguard/commit/5b10ecde5ff47f8e85686c8aa6171d112e66a3bd))
* add delivery governance checks ([eb1b803](https://github.com/devr-tools/codeguard/commit/eb1b8032e9e2a7320c6abc1eb45dca843bc06dd6))
* add local quality precision rules ([dd75b79](https://github.com/devr-tools/codeguard/commit/dd75b79149463d4e9e20c41159c9cb1db69ed002))
* add local quality precision rules ([00c493e](https://github.com/devr-tools/codeguard/commit/00c493e4337d8b2c3c5673d0cc99aeb420ce55b2))
* add observability and operations checks ([3e8713c](https://github.com/devr-tools/codeguard/commit/3e8713cb6ec754eef3dee33ab86457e19654607e))
* add operability, design, and delivery governance checks ([#70](https://github.com/devr-tools/codeguard/issues/70)) ([a52199a](https://github.com/devr-tools/codeguard/commit/a52199a4471fdd093385e3f33ca4cc48dc1c5752))
* add opt-in recommended check defaults ([1680b21](https://github.com/devr-tools/codeguard/commit/1680b21d551c31e86c8f85386c5d5154f95188ad))
* add opt-in recommended check defaults ([#66](https://github.com/devr-tools/codeguard/issues/66)) ([67510e3](https://github.com/devr-tools/codeguard/commit/67510e3c3cb23ce1e22128d987fb3400e161d600))
* add production readiness rule families ([d58e8eb](https://github.com/devr-tools/codeguard/commit/d58e8eb61e418bf8f2b1f00f06a05fcb7f18f464))
* Add production reliability and data-readiness checks across languages ([#67](https://github.com/devr-tools/codeguard/issues/67)) ([c243ead](https://github.com/devr-tools/codeguard/commit/c243ead656a603b1e3bc26896edca6d4da617acb))
* add refused bequest smell detection ([053cb1a](https://github.com/devr-tools/codeguard/commit/053cb1a3d4cfed9c4fc39f638259efa663d26ed3))
* add refused bequest smell detection ([#72](https://github.com/devr-tools/codeguard/issues/72)) ([4423aa4](https://github.com/devr-tools/codeguard/commit/4423aa4d1ff53d96b43000afa47206177b5f7a4e))
* add structural smell quality rules ([eefd8bd](https://github.com/devr-tools/codeguard/commit/eefd8bd2d403f1600e8c2b9fc9bb7ab31e677b78))
* change safety testability refactors ([#69](https://github.com/devr-tools/codeguard/issues/69)) ([9081ad3](https://github.com/devr-tools/codeguard/commit/9081ad3abf666ae1e44faeec0e9611a8f275e80f))
* deepen code smell and maintainability precision checks ([#71](https://github.com/devr-tools/codeguard/issues/71)) ([d554347](https://github.com/devr-tools/codeguard/commit/d5543476080cf35926d26508e6377fd6ca4d7999))
* deepen reliability parity checks ([fc8d769](https://github.com/devr-tools/codeguard/commit/fc8d769fca03a887171cd52ad1c690017d9ef767))
* expand production readiness language coverage ([588bf19](https://github.com/devr-tools/codeguard/commit/588bf19e73579dc566a3510baecb679c69157ce3))
* scaffold change safety checks ([0b313d9](https://github.com/devr-tools/codeguard/commit/0b313d9e93497f5d7f1e04496ee9597095218f6c))


### Bug Fixes

* dogfood production readiness coverage ([1110f4f](https://github.com/devr-tools/codeguard/commit/1110f4f9b452e306e604624c446fbc158c92b234))
* restore change testability detectors ([88f6df0](https://github.com/devr-tools/codeguard/commit/88f6df0d1d17a0f39e29da4c2a0222fed90160c8))
* satisfy strict lint for change safety ([311716c](https://github.com/devr-tools/codeguard/commit/311716c896c01199a45ff2525b92dd0e532c0458))
* satisfy strict lint for operability governance ([abc9fd1](https://github.com/devr-tools/codeguard/commit/abc9fd1e69b83df81d05cad6bdfab10caaeeed15))

## [1.2.2](https://github.com/devr-tools/codeguard/compare/v1.2.1...v1.2.2) (2026-07-25)


### Bug Fixes

* parse pnpm lockfile package entries ([11acbd6](https://github.com/devr-tools/codeguard/commit/11acbd635c1446b8a7237a6cd3403c758c0deb49))
* reduce AI quality false positives ([fd5d103](https://github.com/devr-tools/codeguard/commit/fd5d10319ce0d8cd22705856351936bedcae5662))
* reduce AI quality false positives ([dc61f7e](https://github.com/devr-tools/codeguard/commit/dc61f7e840b5c5b010de864e4cc04c789f6613cf))
* reduce AI quality false positives ([#64](https://github.com/devr-tools/codeguard/issues/64)) ([b80b5c3](https://github.com/devr-tools/codeguard/commit/b80b5c3285a66e596b622cf297538b91e867620a))
* resolve TypeScript pnpm imports ([154adc3](https://github.com/devr-tools/codeguard/commit/154adc3dfd9c2eadf3356b6a1e900486d0300289))
* resolve TypeScript pnpm imports ([#63](https://github.com/devr-tools/codeguard/issues/63)) ([4d3cc59](https://github.com/devr-tools/codeguard/commit/4d3cc597eff62062a1637a04c1dc6b7e4f6f4228))
* satisfy import resolver lint ([fcec2dc](https://github.com/devr-tools/codeguard/commit/fcec2dc2d14420b9ed23df1ab5cd297e3865ce21))

## [1.2.1](https://github.com/devr-tools/codeguard/compare/v1.2.0...v1.2.1) (2026-07-23)


### Bug Fixes

* bound TypeScript semantic analysis to scan corpus ([f473e6e](https://github.com/devr-tools/codeguard/commit/f473e6eacc4329898e134f15c27d18b9a9bba00c))
* bound TypeScript semantic analysis to scan corpus ([#61](https://github.com/devr-tools/codeguard/issues/61)) ([685cf99](https://github.com/devr-tools/codeguard/commit/685cf99f3a49b9502e6b814d86f32c16169b19db))

## [1.2.0](https://github.com/devr-tools/codeguard/compare/v1.1.2...v1.2.0) (2026-07-21)


### Features

* add agent-native repository guardrails ([86f7142](https://github.com/devr-tools/codeguard/commit/86f7142cb3ce8c11e081e28b0f4d61ea40dcdfff))

## [1.1.2](https://github.com/devr-tools/codeguard/compare/v1.1.1...v1.1.2) (2026-07-17)


### Bug Fixes

* (checks) design-cross-language-boundaries ([#55](https://github.com/devr-tools/codeguard/issues/55)) ([91afc88](https://github.com/devr-tools/codeguard/commit/91afc884bec609e2542e1874cb4d19b0c8006a1c))
* **design:** add configurable architecture boundaries ([f6bdb33](https://github.com/devr-tools/codeguard/commit/f6bdb33989faf83a7dfd007474325587614accd5))
* **design:** add configurable architecture boundaries ([#53](https://github.com/devr-tools/codeguard/issues/53)) ([ea5cd93](https://github.com/devr-tools/codeguard/commit/ea5cd936f5f6a6cab6bc4a39b8fe6337dd79330b))
* **design:** satisfy strict lint ([f8c6471](https://github.com/devr-tools/codeguard/commit/f8c647190c79247e190c9922bc9fd7423ef26cb5))

## [1.1.1](https://github.com/devr-tools/codeguard/compare/v1.1.0...v1.1.1) (2026-07-17)


### Bug Fixes

* **mcp:** preserve session across stream disconnects ([a7c5ed2](https://github.com/devr-tools/codeguard/commit/a7c5ed2f60fb51b43769e3574088f870e2bef348))
* **report:** include version in all output formats ([c8c1ae3](https://github.com/devr-tools/codeguard/commit/c8c1ae3a644d669ad60ecc76abaa2a1fb1f4623f))

## [1.1.0](https://github.com/devr-tools/codeguard/compare/v1.0.0...v1.1.0) (2026-07-17)


### Features

* **cpp:** add tooling and security analysis ([b93d681](https://github.com/devr-tools/codeguard/commit/b93d6819a936b11b18fdca5493bb181ef2e6edc1))
* **cpp:** expand checks across all categories ([29ea77e](https://github.com/devr-tools/codeguard/commit/29ea77ec7ad575e9bd875b2387ec655dac437631))
* **cpp:** expand coverage with tooling, security, and dependency analysis ([#50](https://github.com/devr-tools/codeguard/issues/50)) ([16e8146](https://github.com/devr-tools/codeguard/commit/16e8146820aede9480b2358ee9c567f229b30697))

## [1.0.0](https://github.com/devr-tools/codeguard/compare/v0.8.3...v1.0.0) (2026-07-16)


### ⚠ BREAKING CHANGES

* promote performance rules to their own opt-in check section ([#41](https://github.com/devr-tools/codeguard/issues/41))
* the performance rules' ids renamed from quality.* to performance.* (quality.n-plus-one-query -> performance.n-plus-one-query, quality.go.alloc-in-loop -> performance.go.alloc-in-loop, quality.sync-io-in-request-path -> performance.sync-io-in-request-path, quality.unbounded-goroutines-in-loop -> performance.unbounded-goroutines-in-loop, plus the typescript/javascript mirrors and quality.python.sync-io-in-async). Their detect_* toggles moved from quality_rules to performance_rules, and the section is opt-in: set checks.performance: true to keep running these rules, and update any waivers or baselines that reference the old ids.

### Features

* 30% faster scans, enforceable AI-readiness score, and doc-truth rules ([#44](https://github.com/devr-tools/codeguard/issues/44)) ([718dd08](https://github.com/devr-tools/codeguard/commit/718dd085f2ecf11bf663901f0ecb057cf962d133))
* **context:** add AI-and-human-readiness rules and broaden README drift ([6caac9a](https://github.com/devr-tools/codeguard/commit/6caac9a0a1f687c18a280a238b43d77cf834cd23))
* **context:** AI-readiness gate, doc-truth rules, and 30% faster scans ([1f4f130](https://github.com/devr-tools/codeguard/commit/1f4f130ac7e309ba97e737a457f0687b067569b0))
* **context:** AI-readiness gate, doc-truth rules, and 30% faster scans (re-land [#44](https://github.com/devr-tools/codeguard/issues/44)) ([#46](https://github.com/devr-tools/codeguard/issues/46)) ([75b3f02](https://github.com/devr-tools/codeguard/commit/75b3f02c5cfca86eb38d12b8a5c8e7730d733e5b))
* **context:** enforceable legibility threshold and recalibrated score components ([604dd52](https://github.com/devr-tools/codeguard/commit/604dd52cf9503d1db43b34d094f97584c64d9e52))
* **context:** persist repo_legibility score history with report flag ([e54f643](https://github.com/devr-tools/codeguard/commit/e54f6430a3f8312f7ab2a4e108f29607a6b9b6fd))
* **parsers:** wire the Python tree-sitter grammar and upgrade Python N+1 detection ([07a0b24](https://github.com/devr-tools/codeguard/commit/07a0b24c487feead16df877723e9839736f0926a))
* **performance:** add AI-assisted semantic performance lens ([eabd7fe](https://github.com/devr-tools/codeguard/commit/eabd7feb33dcc1d10c02b3798f1a90f434c88b7e))
* **performance:** add diff-only loop-nesting complexity regression rule ([2e8bcef](https://github.com/devr-tools/codeguard/commit/2e8bcef06e819083a569338dee90a3d15a40c57e))
* **performance:** add framework-aware rules for Django, SQLAlchemy, React, and Express ([b49ce7b](https://github.com/devr-tools/codeguard/commit/b49ce7b11801509711972eedd29d1fa2084f1f03))
* **performance:** add loop-hygiene, concurrency, and memory-pressure rules ([54bfca9](https://github.com/devr-tools/codeguard/commit/54bfca96401947cae75ad29ade4dfc904508d126))
* **performance:** add measured budgets and benchmark regression gates ([2c5e4ba](https://github.com/devr-tools/codeguard/commit/2c5e4ba62fc7d928f4e025878078f8d9a4a043ea))
* **performance:** publish a performance_score artifact with history and report flag ([5c5a738](https://github.com/devr-tools/codeguard/commit/5c5a73872340609fa1e13c21bcf11a963e5342c0))
* promote performance rules to their own opt-in check section ([ede8b37](https://github.com/devr-tools/codeguard/commit/ede8b37e4a6ddb2b467bfbe50bc611fa648fd3c6))
* promote performance rules to their own opt-in check section ([#41](https://github.com/devr-tools/codeguard/issues/41)) ([81a575b](https://github.com/devr-tools/codeguard/commit/81a575b60c7404dbec1db45240557112187103bf))
* suggest enabling the performance section in scan output ([81d9d46](https://github.com/devr-tools/codeguard/commit/81d9d46e2f11e0913b8fd8e66d984020aa582476))


### Performance Improvements

* **quality:** hash clone tokens once and roll the window hash ([bbf5b69](https://github.com/devr-tools/codeguard/commit/bbf5b69147e09f77e037a830c834091f95834d80))
* **quality:** route AI check reads through the shared scan corpus ([b271cfb](https://github.com/devr-tools/codeguard/commit/b271cfb6abde0b34378ea7fb63e2bea33730f709))
* **runner:** count lines by scanning bytes instead of allocating ([17a4593](https://github.com/devr-tools/codeguard/commit/17a45937f071d67cde965b454c3e3000df6d0c40))

## [0.8.3](https://github.com/devr-tools/codeguard/compare/v0.8.2...v0.8.3) (2026-07-16)


### Bug Fixes

* **ci:** pin cosign to the v2 line so release signing keeps the .sig/.pem contract ([9ee93c4](https://github.com/devr-tools/codeguard/commit/9ee93c49f54d6d596ed2cbf62bef8f5bc14561b8))
* **ci:** pin cosign to v2 line to unbreak release signing ([#39](https://github.com/devr-tools/codeguard/issues/39)) ([218f078](https://github.com/devr-tools/codeguard/commit/218f078a1162fc3f2480f7e672de971d2aa72057))

## [0.8.2](https://github.com/devr-tools/codeguard/compare/v0.8.1...v0.8.2) (2026-07-16)


### Bug Fixes

* **quality:** keep exec error in coverage-command failure chain ([0666e42](https://github.com/devr-tools/codeguard/commit/0666e42bfcf237310c33dd0a79a23fd50df25b17))
* **security:** surface invalid secret patterns instead of silently dropping them ([5445fde](https://github.com/devr-tools/codeguard/commit/5445fdeb8da732682a9f7f6cccd62ed648eb7ac2))

## [0.8.1](https://github.com/devr-tools/codeguard/compare/v0.8.0...v0.8.1) (2026-07-03)


### Bug Fixes

* **release:** disable PyPI attestations for reusable-workflow publish ([28a147d](https://github.com/devr-tools/codeguard/commit/28a147d14834ae20cf59749fcd6f21f51f02a568))

## [0.8.0](https://github.com/devr-tools/codeguard/compare/v0.7.0...v0.8.0) (2026-07-03)


### Features

* **packaging:** publish codeguard to npm and PyPI ([6a03f0e](https://github.com/devr-tools/codeguard/commit/6a03f0ef2f5a45dd7ffa3b46075e32c6601f997a))
* **packaging:** publish codeguard to npm and PyPI ([#29](https://github.com/devr-tools/codeguard/issues/29)) ([bf39a7a](https://github.com/devr-tools/codeguard/commit/bf39a7a53aea44545a09ec93e3feeead87b707b7))

## [0.7.0](https://github.com/devr-tools/codeguard/compare/v0.6.1...v0.7.0) (2026-07-02)


### Features

* **cli:** add What's New banner and task-grouped menu ([c1df999](https://github.com/devr-tools/codeguard/commit/c1df9990f0255dcadc09aa459da1a7278a06e4df))
* **cli:** What's New banner and task-grouped menu ([#21](https://github.com/devr-tools/codeguard/issues/21)) ([45e56f7](https://github.com/devr-tools/codeguard/commit/45e56f7bcecf1a9edd7d3dd02ca515df58482f1a))
* execute checks-improvement plan tiers 1-6 ([733862f](https://github.com/devr-tools/codeguard/commit/733862f22781b9b1ed32f16c2993f8c92d7e353c))
* **parsers:** tree-sitter TypeScript engine behind parsers.treesitter flag ([5d09161](https://github.com/devr-tools/codeguard/commit/5d09161c908437341994ece22841726da8aacaf4))


### Bug Fixes

* **ci:** extract version from var Number in homebrew validation ([01d7a24](https://github.com/devr-tools/codeguard/commit/01d7a24593da880123f0a50a7c3868467a58b4e0))
* **ci:** extract version from var Number in homebrew validation ([#23](https://github.com/devr-tools/codeguard/issues/23)) ([ec21f3b](https://github.com/devr-tools/codeguard/commit/ec21f3b60404e13647855b5837a53ae7778ec268))
* **version:** make Number a var so ldflags injection works ([6b1ccad](https://github.com/devr-tools/codeguard/commit/6b1ccadbfb0e259d093a81900ccfd2603032e9e4))


### Performance Improvements

* **scan:** parse/read files once, parallelize sections, scope cache ([d181a4f](https://github.com/devr-tools/codeguard/commit/d181a4fc75ec5ffaac62ce867f244f607b514b43))
* **scan:** parse/read files once, parallelize sections, scope cache ([#24](https://github.com/devr-tools/codeguard/issues/24)) ([c79d9ec](https://github.com/devr-tools/codeguard/commit/c79d9ec1bfcaf036466ba588b19c974c06238f95))

## [0.6.1](https://github.com/devr-tools/codeguard/compare/v0.6.0...v0.6.1) (2026-07-01)


### Bug Fixes

* **security:** harden untrusted-input handling and wire lint cleanup ([34c7f87](https://github.com/devr-tools/codeguard/commit/34c7f87c8f6d0bf55917abcb3f5a05e81fcf3669))

## [0.6.0](https://github.com/devr-tools/codeguard/compare/v0.5.0...v0.6.0) (2026-06-30)


### Features

* **security:** hardcoded secret/credential detection + git-history scan ([f2f6c61](https://github.com/devr-tools/codeguard/commit/f2f6c61e171c46eb8af7b554e506c719a077aaf2))
* **security:** hardcoded secret/credential detection + git-history scan ([#17](https://github.com/devr-tools/codeguard/issues/17)) ([c1afab7](https://github.com/devr-tools/codeguard/commit/c1afab751ac0d7d5270ae78bd6936c590fe4a2c5))


### Bug Fixes

* **security:** anchor Slack webhook regex to satisfy CodeQL ([52de05a](https://github.com/devr-tools/codeguard/commit/52de05aec540115392936e6b2caacf91fa4ee466))

## [0.5.0](https://github.com/devr-tools/codeguard/compare/v0.4.0...v0.5.0) (2026-06-19)


### Features

* mcp server features ([2efd47c](https://github.com/devr-tools/codeguard/commit/2efd47c9bd701f5508bb53f33ce3e5af37d0b03d))
* **mcp:** HTTP transport, Devin pack, verified fix, sampling/roots ([#15](https://github.com/devr-tools/codeguard/issues/15)) ([44de1bb](https://github.com/devr-tools/codeguard/commit/44de1bba2adfe040536f2178f6015bc744d4c72e))
* **mcp:** HTTP transport, Devin pack, verified fix, sampling/roots, streaming ([0bfab1d](https://github.com/devr-tools/codeguard/commit/0bfab1d1955fb5010addcf4541b3c82f37c809d5))

## [0.4.0](https://github.com/devr-tools/codeguard/compare/v0.3.0...v0.4.0) (2026-06-18)


### Features

* add OWASP-aware security metadata, trust-policy enforcement ([#10](https://github.com/devr-tools/codeguard/issues/10)) ([358d4ba](https://github.com/devr-tools/codeguard/commit/358d4baf3d07eb87e5d1bce24dcd2384403a3db8))
* feat: add OWASP-aware security metadata, trust-policy enforcement, and quality-check cleanup ([9111a58](https://github.com/devr-tools/codeguard/commit/9111a586c5f006ce8d80a0eccbdcc30fbc324ea4))
* fix cd push security updates ([685a3ff](https://github.com/devr-tools/codeguard/commit/685a3ff4ecce755f25a5d9889ebfee277f8e39d0))

## [0.3.0](https://github.com/devr-tools/codeguard/compare/v0.2.0...v0.3.0) (2026-06-17)


### Features

* add supply chain policies, semantic review, and AI change risk ([9b2dc1f](https://github.com/devr-tools/codeguard/commit/9b2dc1f25893a98e33cdd2d5336d0dd690b67387))
* add supply chain policies, semantic review, and AI change risk ([#8](https://github.com/devr-tools/codeguard/issues/8)) ([3ae9728](https://github.com/devr-tools/codeguard/commit/3ae97289ff8003eb3c071c2ad5697858f2c58419))

## [0.2.0](https://github.com/devr-tools/codeguard/compare/v0.1.0...v0.2.0) (2026-06-16)


### Features

* add contracts check family for API/contract drift detection ([1406bd5](https://github.com/devr-tools/codeguard/commit/1406bd5f4a68b1d7a469c325e79fde6fb536b875))
* feat(inital release) ([b6620e3](https://github.com/devr-tools/codeguard/commit/b6620e3024ffa5326eb06a612b5804270ce8250c))
* **inital release:** initial release ([f7cc9cd](https://github.com/devr-tools/codeguard/commit/f7cc9cdfa030d42666c3b60f886b8938751fd7cd))
* **parsers,security:** deepen pure-Go parsers and add Go/Python taint analysis ([9f42f4a](https://github.com/devr-tools/codeguard/commit/9f42f4a75d9916b0eff040b57edafc09af7b1c64))
* **typescript:** cross-module taint analysis with function summaries ([d8fc21c](https://github.com/devr-tools/codeguard/commit/d8fc21c56f9290e6a23955816bf7a534a118fc14))

## Changelog
