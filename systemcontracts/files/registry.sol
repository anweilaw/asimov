pragma solidity 0.4.25;

import "./library/SafeMath.sol";

/// @dev GenesisOrganization interface
interface GenesisOrganization {
    function existed(address caller) external returns (bool);
}

/// @dev Organization interface
interface Organization {
    function canTransfer(address fromAddress, uint32 assetIndex) external returns(bool);
}


/// @title This is the registry center smart contract.
///  Every organization should register and get a unique organization ID before it can create assets on Asimov.
contract RegistryCenter {
    /// delegate contract address
    address constant delegateAddr = 0x630000000000000000000000000000000000000065;
    /// initialized or not
    bool private initialized;
    /// last assigned organization id, auto incremental
    uint32 lastOrganizationIdAssigned = 0;

    /// @dev struct to keep organization information
    struct OrganizationInfo {
        /// organization id, name and contract address
        uint32 organizationId;
        string organizationName;
        address organizationContract;
        /// template used
        string templateName;
        /// organization status
        /// once registered, the default status of an organization is 'active=true' and 'registered=true'
        bool active;
        bool registered;
        /// the block height when organization status was lastly updated
        uint blockHeight;
        /// assets created by the organization
        uint32[] assetIndexes;
    }

    /// @dev struct to keep template information
    struct TemplateInfo {
        /// template name, which is the primary key
        string templateName;
        /// template status
        /// once registered, the default status of a template is 'active=true' and 'registered=true'
        bool active;
        bool registered;
        /// the block height when template status was lastly updated
        uint blockHeight;
        // organizations created from this template
        uint32[] organizationIds;
    }

    /// @dev struct to keep asset information
    struct AssetInfo {
        /// basic information
        string name;
        string symbol;
        string description;
        /// assetType
        /// divisible = 0, indivisible = 1
        uint32 assetType;
        /// the organization id where this asset belongs to
        uint32 organizationId;
        /// the asset index inside the organization
        uint32 assetIndex;
        /// total amount issued on a divisible asset OR total count issued on an indivisible asset
        uint totalIssued;
        /// history of issued amount on a divisible asset OR voucherId on an indivisible asset
        uint[] issuedHistory;
        /// whether the asset is restricted for circulation
        bool isRestricted;
        // whether the asset is existed
        bool existed;
    }

    // organizationName => bool
    mapping (string => bool) organizationNameMap;
    // organizationId => organizationInfo
    mapping (uint32 => OrganizationInfo) organizationIdInfoMap;
    // templateName => templateInfo
    mapping (string => TemplateInfo) templateNameInfoMap;
    // organizationAddress => organizationId
    mapping (address => uint32) organizationAddressIdMap;
    // assetId => assetInfo
    mapping (uint64 => AssetInfo) assetIdInfoMap;

    // Genesis organization reference
    GenesisOrganization genesisOrganization;

    /// black hole address to destory asset
    address burnAddress;

    /// close the organization
    event CloseOrganizationEvent(address contractAddress);

    function init() public {
        require(!initialized, "it is not allowed to init more than once");
        genesisOrganization = GenesisOrganization(0x630000000000000000000000000000000000000064);
        burnAddress = 0x660000000000000000000000000000000000000000;
        initialized = true;
    }

    /**
     * @dev register an organization to Asimov, should only called by a contract
     *
     * @param organizationName organization name
     * @param templateName template name
     */
    function registerOrganization(string organizationName, string templateName) public returns(uint32) {
        require(bytes(organizationName).length > 0, "invalid organization name");
        require(bytes(templateName).length > 0, "invalid template name");
        require(!organizationNameMap[organizationName], "organization name already existed");

        lastOrganizationIdAssigned = SafeMath.addUint32(lastOrganizationIdAssigned, 1);
        
        OrganizationInfo storage orgInfo = organizationIdInfoMap[lastOrganizationIdAssigned];
        orgInfo.organizationId = lastOrganizationIdAssigned;
        orgInfo.organizationName = organizationName;
        orgInfo.organizationContract = msg.sender;
        orgInfo.templateName = templateName;
        orgInfo.active = true;
        orgInfo.registered = true;
        orgInfo.blockHeight = block.number;
        orgInfo.assetIndexes = new uint32[](0);

        organizationNameMap[organizationName] = true;
        organizationAddressIdMap[msg.sender] = lastOrganizationIdAssigned;

        /// if the template does not exist yet, add it
        TemplateInfo storage tInfo = templateNameInfoMap[templateName];
        tInfo.organizationIds.push(lastOrganizationIdAssigned);
        if (!tInfo.registered) {
            tInfo.templateName = templateName;
            tInfo.active = true;
            tInfo.registered = true;
            tInfo.blockHeight = block.number;
        }
        return lastOrganizationIdAssigned;
    }

    /**
     * @dev update organization name
     *
     * @param organizationName new organization name
     */
    function renameOrganization(string organizationName) public {
        require(bytes(organizationName).length > 0, "invalid organization name");
        require(!organizationNameMap[organizationName], "organization name already existed");

        uint32 organizationId = organizationAddressIdMap[msg.sender];
        require(organizationId > 0, "you have not registered organization");

        OrganizationInfo storage orgInfo = organizationIdInfoMap[organizationId];
        if (orgInfo.registered) {
            organizationNameMap[orgInfo.organizationName] = false;
            orgInfo.organizationName = organizationName;
            organizationNameMap[organizationName] = true;
        }
    }

    /**
     * @dev update organization status
     *
     * @param isActive the new status
     */
    function updateOrganizationStatus(bool isActive) public {
        uint32 organizationId = organizationAddressIdMap[msg.sender];
        require(organizationId > 0, "you have not registered organization");

        OrganizationInfo storage orgInfo = organizationIdInfoMap[organizationId];
        require(orgInfo.registered, "organization has not registered");

        orgInfo.active = isActive;
        orgInfo.blockHeight = block.number;
    }

    /**
     * @dev close organization
     */
    function closeOrganization() public {
        uint32 organizationId = organizationAddressIdMap[msg.sender];
        require(organizationId > 0, "you have not registered organization");

        OrganizationInfo storage orgInfo = organizationIdInfoMap[organizationId];
        require(orgInfo.registered, "organization has not registered");

        orgInfo.active = false;
        orgInfo.registered = false;
        organizationNameMap[orgInfo.organizationName] = false;

        /// this event is used by DAO
        /// when DAO contract is self destructed, AScan needs a signal
        emit CloseOrganizationEvent(msg.sender);
    }

    /**
     * @dev deactivate a template
     *
     * @param templateName the template to deactivate
     */
    function deactivateTemplate(string templateName) public {
        require(genesisOrganization.existed(msg.sender), "not allowed to deactivate a template");
        TemplateInfo storage tInfo = templateNameInfoMap[templateName];
        require(tInfo.registered, "no such template");

        tInfo.active = false;
        tInfo.blockHeight = block.number;
    }

    /**
     * @dev create an asset on Asimov chain, this function is called by 'flow.createAsset()' instruction
     *
     * @param organizationAddress organization address
     * @param assetType divisible = 0, indivisible = 1
     * @param assetIndex asset index in the organization
     * @param amountOrVoucherId amount or voucherId of asset
     * @param isRestricted restricted or not
     * @return success or not
     */
    function create(address organizationAddress, uint32 assetType, uint32 assetIndex, uint amountOrVoucherId, bool isRestricted)
        public 
        returns (bool) 
    {
        require(msg.sender == delegateAddr, "invalid caller");
        require(0 == assetType || 1 == assetType, "invalid assetType");
        uint32 organizationId = organizationAddressIdMap[organizationAddress];
        require(organizationId > 0, "you have not registered organization");

        OrganizationInfo storage orgInfo = organizationIdInfoMap[organizationId];
        if (!orgInfo.registered || !orgInfo.active) {
            return false;
        }

        TemplateInfo storage tInfo = templateNameInfoMap[orgInfo.templateName];
        if (!tInfo.active) {
            return false;
        }

        uint64 assetId = generateAssetID(organizationId, assetIndex);
        AssetInfo storage assetInfo = assetIdInfoMap[assetId];
        if (assetInfo.existed) {
            return false;
        }

        assetInfo.assetType = assetType;
        assetInfo.assetIndex = assetIndex;
        assetInfo.organizationId = organizationId;
        assetInfo.isRestricted = isRestricted;
        assetInfo.existed = true;
        orgInfo.assetIndexes.push(assetIndex);

        if (0 == getDivisibleBit(assetType)) {
            assetInfo.totalIssued = amountOrVoucherId;
        } else if (1 == getDivisibleBit(assetType)) {
            assetInfo.totalIssued = 1;
        }
        assetInfo.issuedHistory.push(amountOrVoucherId);

        return true;
    }

    /**
     * @dev save asset basic information, should be called after `flow.createAsset()`
     *
     * @param assetIndex asset index in the organization
     * @param name asset name
     * @param symbol asset symbol
     * @param description description of an asset
     */
    function newAssetInfo(uint32 assetIndex, string name, string symbol, string description) public {
        uint32 organizationId = organizationAddressIdMap[msg.sender];
        require(organizationId > 0, "you have not registered organization");

        OrganizationInfo storage orgInfo = organizationIdInfoMap[organizationId];
        require(orgInfo.registered, "organization hasn't registered");
        require(orgInfo.active, "organization is inactive");

        TemplateInfo storage tInfo = templateNameInfoMap[orgInfo.templateName];
        require(tInfo.active, "template is inactive");

        uint64 assetId = generateAssetID(organizationId, assetIndex);
        AssetInfo storage assetInfo = assetIdInfoMap[assetId];
        require(assetInfo.existed, "asset does not exist");

        assetInfo.name = name;
        assetInfo.symbol = symbol;
        assetInfo.description = description;
    }

    /**
     * @dev mint more asset, this function is called by 'flow.mintAsset()' instruction
     *
     * @param organizationAddress organization address
     * @param assetIndex asset index in the organization
     * @param amountOrVoucherId amount or voucherId of asset
     */
    function mintAsset(address organizationAddress, uint32 assetIndex, uint amountOrVoucherId) public {
        require(msg.sender == delegateAddr, "invalid caller");
        uint32 organizationId = organizationAddressIdMap[organizationAddress];
        require(organizationId > 0, "you have not registered organization");

        OrganizationInfo storage orgInfo = organizationIdInfoMap[organizationId];
        require(orgInfo.registered, "organization hasn't registered");
        require(orgInfo.active, "organization is inactive");

        TemplateInfo storage tInfo = templateNameInfoMap[orgInfo.templateName];
        require(tInfo.active, "template is inactive");

        uint64 assetId = generateAssetID(organizationId, assetIndex);
        AssetInfo storage assetInfo = assetIdInfoMap[assetId];
        require(assetInfo.existed, "asset does not exist");

        if (0 == getDivisibleBit(assetInfo.assetType)) {
            assetInfo.totalIssued = SafeMath.add(assetInfo.totalIssued, amountOrVoucherId);
        } else if (1 == getDivisibleBit(assetInfo.assetType)) {
            assetInfo.totalIssued = SafeMath.add(assetInfo.totalIssued, 1);
        }
        assetInfo.issuedHistory.push(amountOrVoucherId);
    }

    /**
     * @dev burn asset, assets can only be burned by the issuer
     */
    function burnAsset() public payable {
        require(msg.value > 0, "value must be bigger than 0");

        uint32 organizationId = organizationAddressIdMap[msg.sender];
        require(organizationId > 0, "you have not registered organization");

        OrganizationInfo storage orgInfo = organizationIdInfoMap[organizationId];
        require(orgInfo.registered, "organization hasn't registered");
        require(orgInfo.active, "organization is inactive");

        TemplateInfo storage tInfo = templateNameInfoMap[orgInfo.templateName];
        require(tInfo.active, "template is inactive");

        AssetInfo storage assetInfo = assetIdInfoMap[uint64(bytes8(msg.assettype%2**64))];
        require(assetInfo.existed, "asset does not exist");

        require(organizationId == assetInfo.organizationId, "not authorized");

        if (0 == getDivisibleBit(assetInfo.assetType)) {
            require(assetInfo.totalIssued >= msg.value, "invalid amount");
            assetInfo.totalIssued = SafeMath.sub(assetInfo.totalIssued, msg.value);
        } else if (1 == getDivisibleBit(assetInfo.assetType)) {
            require(assetInfo.totalIssued >= 1, "invalid call");
            assetInfo.totalIssued = SafeMath.sub(assetInfo.totalIssued, 1);
        }
        burnAddress.transfer(msg.value, msg.assettype);
    }

    /**
     * @dev get asset basic information by assetId
     *
     * @param organizationId the organization Id
     * @param assetIndex asset index of an organization, which is maintained by the organization itself
     * @return existed + name + symbol + description + total issued
     */
    function getAssetInfoByAssetId(uint32 organizationId, uint32 assetIndex)
        public
        view
        returns(bool, string, string, string, uint, uint[])
    {
        uint64 assetId = generateAssetID(organizationId, assetIndex);
        AssetInfo storage assetInfo = assetIdInfoMap[assetId];
        if (!assetInfo.existed) {
            return (false, "", "", "", 0, new uint[](0));
        }
        return (true, assetInfo.name, assetInfo.symbol, assetInfo.description, assetInfo.totalIssued, assetInfo.issuedHistory);
    }

    /**
     * @dev check whether an asset can be transferred
     *
     * @param organizationId the organization Id
     * @param assetIndex asset index of an organization, which is maintained by the organization itself
     * @return yes or no
     */
    function canTransfer(uint32 organizationId, uint32 assetIndex) public view returns (bool) {
        OrganizationInfo storage orgInfo = organizationIdInfoMap[organizationId];
        if (!orgInfo.registered || !orgInfo.active) {
            return false;
        }
        uint64 assetId = generateAssetID(organizationId, assetIndex);
        if (!assetIdInfoMap[assetId].existed) {
            return false;
        }

        TemplateInfo storage tInfo = templateNameInfoMap[orgInfo.templateName];
        if (!tInfo.registered) {
            return false;
        }

        uint templateBlockHeight = tInfo.blockHeight;
        uint orgBlockHeight = orgInfo.blockHeight;
        if (!tInfo.active && templateBlockHeight > orgBlockHeight) {
            return false;
        }
        return true;
    }

    /**
     * @dev check whether the asset is restricted for circulation
     *
     * @param organizationId the organization Id
     * @param assetIndex asset index of an organization, which is maintained by the organization itself
     * @return yes or no
     */
    function isRestrictedAsset(uint32 organizationId, uint32 assetIndex) public view returns(bool, bool) {
        AssetInfo storage assetInfo = assetIdInfoMap[generateAssetID(organizationId, assetIndex)];
        if (!assetInfo.existed) {
            return (false, false);
        }
        return (true, assetInfo.isRestricted);
    }

    /**
     * @dev remove circulation restriction of an asset
     *
     * @param assetIndex asset index of an organization, which is maintained by the organization itself
     */
    function removeAssetRestriction(uint32 assetIndex) public {
        uint32 organizationId = organizationAddressIdMap[msg.sender];
        require(organizationId > 0, "you have not registered organization");

        OrganizationInfo storage orgInfo = organizationIdInfoMap[organizationId];
        require(orgInfo.registered, "you have not registered organization");
        require(orgInfo.active, "organization is inactive");

        TemplateInfo storage tInfo = templateNameInfoMap[orgInfo.templateName];
        require(tInfo.active, "template is inactive");

        uint64 assetId = generateAssetID(organizationId, assetIndex);
        AssetInfo storage assetInfo = assetIdInfoMap[assetId];
        require(assetInfo.existed, "you have not create the asset");

        assetInfo.isRestricted = false;
    }

    /**
     * @dev get organization address by organization id
     *
     * @param organizationId the organization Id
     * @param assetIndex asset index of an organization, which is maintained by the organization itself
     * @return organization address
     */
    function getOrganizationAddressById(uint32 organizationId, uint32 assetIndex)
        public
        view
        returns(address)
    {
        OrganizationInfo storage orgInfo = organizationIdInfoMap[organizationId];
        if (!orgInfo.registered || !orgInfo.active) {
            return 0x0;
        }
        uint64 assetId = generateAssetID(organizationId, assetIndex);
        if (!assetIdInfoMap[assetId].existed) {
            return 0x0;
        }

        TemplateInfo storage tInfo = templateNameInfoMap[orgInfo.templateName];
        if (!tInfo.registered || !tInfo.active) {
            return 0x0;
        }

        return orgInfo.organizationContract;
    }

    /**
     * @dev get organization id
     *
     * @return (true/false + organization id)
     */
    function getOrganizationId() public view returns(bool, uint32) {
        uint32 organizationId = organizationAddressIdMap[msg.sender];
        OrganizationInfo storage orgInfo = organizationIdInfoMap[organizationId];
        if (!orgInfo.registered) {
            return (false, 0);
        }
        return (true, organizationId);
    }

    /**
     * @dev find asset information by asset index
     *
     * @param assetIndex asset index of an organization, which is maintained by the organization itself
     * @return (assetType + organizationId + existed)
     */
    function findAsset(uint32 assetIndex) public view returns(uint32, uint32, bool) {
        uint32 organizationId = organizationAddressIdMap[msg.sender];
        OrganizationInfo storage orgInfo = organizationIdInfoMap[organizationId];
        if (!orgInfo.registered) {
            return (0, 0, false);
        }

        uint64 assetId = generateAssetID(organizationId, assetIndex);
        AssetInfo storage assetInfo = assetIdInfoMap[assetId];
        if (!assetInfo.existed) {
            return (0, 0, false);
        }

        return (assetInfo.assetType, assetInfo.organizationId, true);
    }

    /**
     * @dev check whether the calling organization contract has registered
     *
     * @return yes or no
     */
    function hasRegistered() public view returns(bool) {
        uint32 organizationId = organizationAddressIdMap[msg.sender];
        OrganizationInfo storage orgInfo = organizationIdInfoMap[organizationId];
        return orgInfo.registered;
    }

    /**
     * @dev batch get organization addresses by asset ids
     *
     * @return organization addresses
     */
    function getOrganizationAddressesByAssets(uint64[] assetIds)
        public
        view
        returns(address[])
    {
        uint length = assetIds.length;
        if (length == 0) {
            return new address[](0);
        }

        address[] memory addresses = new address[](length);
        for (uint i = 0; i < length; i++) {
            AssetInfo storage assetInfo = assetIdInfoMap[assetIds[i]];
            if (assetInfo.existed) {
                OrganizationInfo storage orgInfo = organizationIdInfoMap[assetInfo.organizationId];
                addresses[i] = orgInfo.organizationContract;
            }
        }
        return addresses;
    }

    /**
     * @dev generate the asset id from organization id and asset index
     */
    function generateAssetID(uint32 a, uint32 b) internal pure returns (uint64) {
        return uint64(a) << 32 | uint64(b);
    }

    /**
     * @dev internal method: get property of isDivisible from assetType
     */
    function getDivisibleBit(uint32 assetType) internal pure returns(uint32) {
        uint32 lastFourBits = assetType & 0xf;
        return lastFourBits & 1;
    }

}




