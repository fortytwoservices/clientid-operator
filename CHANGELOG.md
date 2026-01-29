# Changelog

## 1.0.0 (2026-01-29)


### Features

* add dockerfile and kubernetes deployment manifests ([3f0fe27](https://github.com/fortytwoservices/clientid-operator/commit/3f0fe279858df30c5814c4a19ece9f87c68ebfec))
* add dockerfile and kubernetes deployment manifests ([46f5d86](https://github.com/fortytwoservices/clientid-operator/commit/46f5d86cd376d5a42404a0dba47d398a0af8a189))
* add initial controller code ([7b62436](https://github.com/fortytwoservices/clientid-operator/commit/7b62436481a93ee761be4bd41f16b684a50cffaa))
* add the ability for the operator to sync roleassignments princi… ([29043cc](https://github.com/fortytwoservices/clientid-operator/commit/29043cce4ade6324307b982e0a5a6d185b2cec8d))
* add the ability for the operator to sync roleassignments principal ids to respective applications as well ([3b59c1f](https://github.com/fortytwoservices/clientid-operator/commit/3b59c1fbe0662d777df4287b632b6af262fa336a))
* check after 60 secs if update was required, check for needed changes every 2 minutes ([374fa42](https://github.com/fortytwoservices/clientid-operator/commit/374fa42559b89af7c4f90fcbf8707325bd36a28a))
* ensure operator also retries periodically ([84184e4](https://github.com/fortytwoservices/clientid-operator/commit/84184e4dad896b67386aad47a923434ec2ca7106))
* make sure the operator retries the sync initially if it fails o… ([fb83afa](https://github.com/fortytwoservices/clientid-operator/commit/fb83afa11cdd1c36b5a2c8acc54af87058298331))
* make sure the operator retries the sync initially if it fails or tries too early ([b64ca24](https://github.com/fortytwoservices/clientid-operator/commit/b64ca24a77dddcb775b30bddfbd9d4f49cdaff5e))
* reconcile every 2 minutes ([a56d7fe](https://github.com/fortytwoservices/clientid-operator/commit/a56d7fe3a9ca5910f7f4997deff8a3078d425958))
* small enhancements to logging ([0e2436e](https://github.com/fortytwoservices/clientid-operator/commit/0e2436ef5af82a90bee5aad4fce5359a574462e5))
* small enhancements to logging ([f085f46](https://github.com/fortytwoservices/clientid-operator/commit/f085f46ac95b981e52ae4ffc522ca008f4092f91))
* update dockerfiel to use golang 1.23 as base ([86e0626](https://github.com/fortytwoservices/clientid-operator/commit/86e0626b6dc2bdab5175189371cb93ac3fb72229))
* update dockerfiel to use golang 1.23 as base ([6f24ab2](https://github.com/fortytwoservices/clientid-operator/commit/6f24ab2e826ecefece9cda7d5d4f788707b6dec9))
* update operator to patch deployments to rollout restart after service account updates ([ea52ce4](https://github.com/fortytwoservices/clientid-operator/commit/ea52ce413410c5a72c4c693423b4d7fbefca9591))
* update readme and controller comments ([655dc4e](https://github.com/fortytwoservices/clientid-operator/commit/655dc4e5e47912d7700f78dfddeb82194de9a469))
* update readme and controller comments ([5460fef](https://github.com/fortytwoservices/clientid-operator/commit/5460fef29fa4c86a352d012037e15d2510e76083))
* update readme to reflect prereqs and general information ([4a215a5](https://github.com/fortytwoservices/clientid-operator/commit/4a215a539c2c4f81e20be696340540ba62ed9187))
* Update README.md ([ca89e1a](https://github.com/fortytwoservices/clientid-operator/commit/ca89e1a8773404eb97af8a38d5a18fcd96d7fcc8))
* Update README.md ([c37e508](https://github.com/fortytwoservices/clientid-operator/commit/c37e5089a08afbda253da4e873299ce07bc788d0))
* Update README.md ([8abe5ce](https://github.com/fortytwoservices/clientid-operator/commit/8abe5ceb4b05f07e5950e61e18b8ca692ab32deb))
* update reconcilation ([7857abb](https://github.com/fortytwoservices/clientid-operator/commit/7857abbcdd190daf91047b4e0330a16e70e7a20d))
* Updated code to support new namespaced resources ([be67afb](https://github.com/fortytwoservices/clientid-operator/commit/be67afb3c8ea4101b0537a30f71e386af93fcc5d))


### Bug Fixes

* add controller test case ([d0bbc61](https://github.com/fortytwoservices/clientid-operator/commit/d0bbc61e38ecc22698a34625383f269b0535e08e))
* correct go.mod ([9687524](https://github.com/fortytwoservices/clientid-operator/commit/9687524bbccc3c94e5de9adfe7749870a5a8e58f))
* correct naming ([19f495c](https://github.com/fortytwoservices/clientid-operator/commit/19f495c195af57cbac4d99277f38b67c59f637a7))
* update go.mod ([20cd113](https://github.com/fortytwoservices/clientid-operator/commit/20cd113e702891317feb60ea55da69b100cd11ab))
