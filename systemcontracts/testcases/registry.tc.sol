pragma solidity 0.4.25;

import "../utils/test_tool.sol";

interface Registry {
	function init() external;
	function registerOrganization(string organizationName, string templateName) external returns(uint32);
	function renameOrganization(string organizationName) external;
	function updateOrganizationStatus(bool status) external;
	function deactivateTemplate(string templateName) external;
	function create(address organizationAddress, uint32 assetType, uint32 assetIndex, uint amountOrVoucherId, bool isRestricted) external view returns (bool);
	function newAssetInfo(uint32 assetIndex, string name, string symbol, string description) external;
	function mintAsset(address organizationAddress, uint32 assetIndex, uint amountOrVoucherId) external;
	function burnAsset() external payable;
	function getAssetInfoByAssetId(uint32 organizationId, uint32 assetIndex) external view returns(bool, string, string, string, uint, uint[]);
	function canTransfer(uint32 organizationId, uint32 assetIndex) external view returns (bool);
	function isRestrictedAsset(uint32 organizationId, uint32 assetIndex) external view returns(bool, bool);
	function removeAssetRestriction(uint32 assetIndex) external;
	function getOrganizationAddressById(uint32 organizationId, uint32 assetIndex) external view returns(address);
	function getOrganizationId() external view returns(bool, uint32);
	function findAsset(uint32 assetIndex) external view returns(uint32, uint32, bool);
	function hasRegistered() external view returns(bool);
	function getOrganizationAddressesByAssets(uint64[] assetIds) external view returns(address[]);
}

contract TestRegistry {
	Registry internal registry;

	event LogResult(bool);

	function setUp() internal {
		registry = Registry(0x630000000000000000000000000000000000000065);
	}

	function testInit() public {
		setUp();
		registry.init();
	}

	function testRegisterOrganization() public {
		setUp();

		/// success
		registry.registerOrganization("organization1", "template1");
		/// register the same organization name
		registry.registerOrganization("organization1", "template1");
		/// organization name and template name can not be empty string
		registry.registerOrganization("", "b");
		registry.registerOrganization("a", "");
		registry.registerOrganization("", "");
	}

	function testRenameOrganization() public {
		setUp();

		registry.registerOrganization("aaa", "bbb");
		registry.registerOrganization("ccc", "bbb");

		/// organization name can not be empty string
		registry.renameOrganization("");
		/// organization name "ccc" has existed
		registry.renameOrganization("ccc");

		registry.renameOrganization("ddd");
	}

	function testUpdateOrganizationStatus() public {
		setUp();

		registry.registerOrganization("aaa", "bbb");

		registry.updateOrganizationStatus(true);
		registry.updateOrganizationStatus(false);
	}

	function testDeactivateTemplate() public {
		setUp();

		registry.registerOrganization("aaa", "bbb");
		/// only member of genesis organization can deactive template
		registry.deactivateTemplate("bbb");
	}

	function testCreate() public {
		setUp();

		registry.registerOrganization("aaa", "bbb");

		bool result = registry.create(this, 0, 1, 100000000, true);
		emit LogResult(result);

		/// failed; asset index is existing
		result = registry.create(this, 0, 1, 100000000, true);
		emit LogResult(result);

		/// failed; amount is too big
		result = registry.create(this, 0, 1, 0xffffffff, true);
		emit LogResult(result);
	}

	function testNewAssetInfo() public {
		setUp();

		registry.registerOrganization("aaa", "bbb");
		registry.create(this, 0, 1, 100000000, true);
		registry.newAssetInfo(1, "btc", "BTC", "this is btc");
		/// failed; asset index is not existing
		registry.newAssetInfo(2, "btc", "BTC", "this is btc");
	}

	function testMintAsset() public {
		setUp();

		registry.registerOrganization("aaa", "bbb");
		registry.create(this, 0, 1, 100000000, true);
		registry.mintAsset(this, 1, 200000000);
		/// failed; asset index is not existing
		registry.mintAsset(this, 2, 200000000);
	}

	function testBurnAsset() public payable {
		setUp();

		registry.registerOrganization("aaa", "bbb");
		registry.create(this, 0, 1, 100000000, true);
		registry.burnAsset();
		/// failed; the amount to burn is too much
		registry.burnAsset(1, 10000000000);	
		/// failed; asset index is not existing
		registry.burnAsset(10, 999);
	}

	function testGetAssetInfoByAssetId() public {
		setUp();

		uint32 organizationId = registry.registerOrganization("aaa", "bbb");
		registry.create(this, 0, 1, 100000000, true);
		bool isFind;
		(isFind, , , , , ) = registry.getAssetInfoByAssetId(organizationId, 1);
		emit LogResult(isFind);

		/// failed; invalid organizationId
		(isFind, , , , , ) = registry.getAssetInfoByAssetId(organizationId+1, 1);
		emit LogResult(isFind);

		/// failed; invalid asset index
		(isFind, , , , , ) = registry.getAssetInfoByAssetId(organizationId, 2);
		emit LogResult(isFind);
	}

	function testCanTransfer() public {
		setUp();

		uint32 organizationId = registry.registerOrganization("aaa", "bbb");
		registry.create(this, 0, 1, 100000000, true);
		bool canTransfer = registry.canTransfer(organizationId, 1);
		emit LogResult(canTransfer);
	}

	function testIsRestrictedAsset() public {
		setUp();

		uint32 organizationId = registry.registerOrganization("aaa", "bbb");
		registry.create(this, 0, 1, 100000000, true);
		bool issRestricted;
		(issRestricted, ) = registry.isRestrictedAsset(organizationId, 1);
		emit LogResult(issRestricted);
	}

	function testUpdateRestrictionProperty() public {
		setUp();

		registry.registerOrganization("aaa", "bbb");
		registry.create(this, 0, 1, 100000000, true);
		registry.updateRestrictionProperty(1);
		/// failed; invalid asset index
		registry.updateRestrictionProperty(1);
	}

	function testGetOrganizationAddressById() public {
		setUp();

		uint32 organizationId = registry.registerOrganization("aaa", "bbb");
		registry.create(this, 0, 1, 100000000, true);
		/// failed; unknow address 0x66b28f9dd1cf6314a8b5d691aeec6c6eaf456cbd9a
		address organizationAddress = registry.getOrganizationAddressById(organizationId, 1);
		assertTrue(0x0 != organizationAddress)
	}

	function testGetOrganizationId() public {
		setUp();

		uint32 organizationId = registry.registerOrganization("aaa", "bbb");
		
		uint32 id = registry.getOrganizationId();
		if (organizationId == id) {
			emit LogResult(true);
		} else {
			emit LogResult(false);
		}
	}

	function testFindAsset() public {
		setUp();

		uint32 organizationId = registry.registerOrganization("aaa", "bbb");
		uint assetType = 0;
		registry.create(this, 0, 1, 100000000, true);
		uint32 a;
		uint32 b;
		bool c;
		(a, b, c) = registry.findAsset(1);

		if (a == assetType && organizationId == b) {
			emit LogResult(true);
		} else {
			emit LogResult(false);
		}
	}

	function testHasRegistered() public {
		setUp();

		uint32 organizationId = registry.registerOrganization("aaa", "bbb");
		bool registered;
		if (organizationId > 0) {
			registered = true;
		}

		bool a = registry.hasRegistered();
		if (registered == a) {
			emit LogResult(true);
		} else {
			emit LogResult(false);
		}
	}

	function testGetOrganizationAddressesByAssets() public {
		setUp();

		uint32 organizationId = registry.registerOrganization("aaa", "bbb");
		registry.create(this, 0, 1, 100000000, true);
		registry.create(this, 0, 2, 100000000, true);

		uint64[] memory assetIds = new uint64[](2);
		assetIds[0] = 4294967297;
		assetIds[1] = 4294967298;
		address[] memory organizationAddresses = registry.getOrganizationAddressesByAssets(assetIds);
		if (this == organizationAddresses[0] && this == organizationAddresses[1]) {
			emit LogResult(true);
		} else {
			emit LogResult(false);
		}
	}

}




