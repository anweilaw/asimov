pragma solidity 0.4.25;

/// @dev Genesis Oraganization interface
interface GenesisOrganization{
    function existed(address caller) external returns (bool);
}

/// @title This is the smart contract to support the template review feature
///  It also saves all template bytecodes in Flow
contract TemplateWarehouse {
    address constant delegateAddr = 0x630000000000000000000000000000000000000067;
    /// @dev template status enumerations
    enum Status { Submitted, Approved, NonExisted, Disabled }

    bool private initialized;

    /// struct to keep template information
    struct Template {
        /// file's key
        bytes32 key;
        /// template status
        Status status;
        /// approvors and rejecters for this specific template
        address[] approvers;
        address[] rejecters;
        /// owner of the template
        address owner;
        /// template creation time
        uint createTime;
        /// whether the template exists
        bool exist;
    }

    /// struct to keep category information
    /// Note every template belongs to one category
    struct Category {
        /// templateName => Template
        mapping (string => Template) templates;
        /// all template names in this category
        string[] names;
        /// approved template names in this category
        string[] approvedNames;
        /// whether the category exists
        bool exist;
    }

    // categoryId => Category
    mapping (uint16 => Category) categories;

    // Genesis Organization reference
    GenesisOrganization genesisOrganization;

    uint8 public constant THRESHOLD = 1;
    uint8 public constant ALL_APPROVER = 1;

    event ContractTemplateEvent(uint16 category, string name, address owner, bytes32 key, uint8 approvers, uint8 rejecters, uint8 allApprover, Status status);

    function init() public {
        require(!initialized, "it is not allowed to init more than once");
        genesisOrganization = GenesisOrganization(0x630000000000000000000000000000000000000064);
        initialized = true;
    }

    /// @dev submit a new template for review
    /// @param _category template category
    /// @param _name template name
    /// @param _key template key
    function create(uint16 _category, string _name, bytes32 _key) public {
        require(_category > 0);
        require(bytes(_name).length > 0 && bytes(_name).length <= 128, "invalid length of template name");
        require(_key.length > 0);

        Category storage c = categories[_category];

        if (c.exist) {
             require(!c.templates[_name].exist, "template name already exists");
        } else {
            /// create a new category if not existed
            c.exist = true;
            c.names = new string[](0);
            c.approvedNames = new string[](0);
        }

        /// create the template
        Template storage t = c.templates[_name];
        t.key = _key;
        t.status = Status.Submitted;
        t.exist = true;
        t.approvers = new address[](0);
        t.rejecters = new address[](0);
        t.owner = msg.sender;
        t.createTime = block.timestamp;

        c.names.push(_name);
        c.templates[_name] = t;

        /// in test environment, a template will be approved automatically after submission
        /// @notice this should be commentted out in PROD environment
        approveForTest(_category, _name);
    }

    /// @dev modifier to support permission control
    modifier actionAuth(uint16 _category, string _name) {
        require(genesisOrganization.existed(msg.sender), "not allowed");
        require(categories[_category].exist, "category not existed");
        require(categories[_category].templates[_name].exist, "template not existed");

        _;
    }

    /// @dev approve for a template
    /// @param _category template category
    /// @param _name template name
    function approve(uint16 _category, string _name) private actionAuth(_category, _name){
        Category storage c = categories[_category];
        Template storage t = c.templates[_name];
        require(t.status == Status.Submitted, "wrong template status");
        require(!addressExist(t.approvers, msg.sender), "not a qualified voter");
        require(!addressExist(t.rejecters, msg.sender), "not a qualified voter");

        require(t.approvers.length < 128);

        t.approvers.push(msg.sender);

        if (t.approvers.length >= THRESHOLD) {
            t.status = Status.Approved;
            c.approvedNames.push(_name);
            deleteName(_category, _name);
        }

        emit ContractTemplateEvent(_category, _name, t.owner, t.key, uint8(t.approvers.length), uint8(t.rejecters.length), ALL_APPROVER, t.status);
    }

    /// @dev this function is used to support automatic approve in test environment
    /// @param _category template category
    /// @param _name template name
    function approveForTest(uint16 _category, string _name) private {
        Category storage c = categories[_category];
        Template storage t = c.templates[_name];
        require(t.status == Status.Submitted, "wrong template status");
        require(!addressExist(t.approvers, msg.sender), "not a qualified voter");
        require(!addressExist(t.rejecters, msg.sender), "not a qualified voter");

        require(t.approvers.length < 128);

        t.approvers.push(msg.sender);

        if (t.approvers.length >= THRESHOLD) {
            t.status = Status.Approved;
            c.approvedNames.push(_name);
            deleteName(_category, _name);
        }

        emit ContractTemplateEvent(_category, _name, t.owner, t.key, uint8(t.approvers.length), uint8(t.rejecters.length), ALL_APPROVER, t.status);
    }

    /// @dev reject a template
    /// @param _category template category
    /// @param _name template name
    function reject(uint16 _category, string _name) private actionAuth(_category, _name){
        Category storage c = categories[_category];
        Template storage t = c.templates[_name];
        require(t.status == Status.Submitted, "wrong template status");
        require(!addressExist(t.approvers, msg.sender), "not a qualified voter");
        require(!addressExist(t.rejecters, msg.sender), "not a qualified voter");

        require(t.rejecters.length < 128);

        t.rejecters.push(msg.sender);

        if (t.rejecters.length > (ALL_APPROVER - THRESHOLD)) {
            deleteTemplate(_category, _name);
        }
    }

    /// @dev get total counts of submitted templates
    /// @param _category template category
    function getSubmittedTemplatesCount(uint16 _category) public view returns (uint256) {
        Category memory c = categories[_category];
        if (!c.exist) {
            return 0;
        }
        return c.names.length;
    }

    /// @dev get a submitted template information by index
    /// @param _category template category
    /// @param index position of the template in the cateogry
    function getSubmittedTemplate(uint16 _category, uint256 index) public view returns (string, bytes32, uint, uint8, uint8, uint8, Status) {
        Category storage c = categories[_category];
        if (!c.exist) {
            return ("", "", 0, 0, 0, ALL_APPROVER, Status.NonExisted);
        }
        if (c.names.length <= 0) {
            return ("", "", 0, 0, 0, ALL_APPROVER, Status.NonExisted);
        }
        string memory name = c.names[index];
        if (!c.templates[name].exist) {
            return ("", "", 0, 0, 0, ALL_APPROVER, Status.NonExisted);
        }

        Template memory t = c.templates[name];
        return (name, t.key, t.createTime, uint8(t.approvers.length), uint8(t.rejecters.length), ALL_APPROVER, t.status);
    }

    /// @dev get total counts of approved templates
    /// @param _category template category
    function getApprovedTemplatesCount(uint16 _category) public view returns (uint256) {
        Category memory c = categories[_category];
        if (!c.exist) {
            return 0;
        }
        return c.approvedNames.length;
    }

    /// @dev get an approved template information by index
    /// @param _category template category
    /// @param index position of the template in the cateogry
    function getApprovedTemplate(uint16 _category, uint256 index) public view returns (string, bytes32, uint, uint8, uint8, uint8, Status) {
        Category storage c = categories[_category];
        if (!c.exist) {
            return ("", "", 0, 0, 0, ALL_APPROVER, Status.NonExisted);
        }
        if (c.approvedNames.length <= 0) {
            return ("", "", 0, 0, 0, ALL_APPROVER, Status.NonExisted);
        }
        string memory name = c.approvedNames[index];
        if (!c.templates[name].exist) {
            return ("", "", 0, 0, 0, ALL_APPROVER, Status.NonExisted);
        }

        Template memory t = c.templates[name];
        return (name, t.key,  t.createTime, uint8(t.approvers.length), uint8(t.rejecters.length), ALL_APPROVER, t.status);
    }

    /// @dev get an approved template information by name
    /// @param _category template category
    /// @param name template name
    function getTemplate(uint16 _category, string name) public view returns (string, bytes32, uint, uint8, uint8, uint8, Status) {
        Category storage c = categories[_category];
        if (!c.exist) {
            return ("", "", 0, 0, 0, ALL_APPROVER, Status.NonExisted);
        }
        if (!c.templates[name].exist) {
            return ("", "", 0, 0, 0, ALL_APPROVER, Status.NonExisted);
        }
        Template memory t = c.templates[name];
         return (name, t.key,  t.createTime, uint8(t.approvers.length), uint8(t.rejecters.length), ALL_APPROVER,t.status);
    }

    /// @dev disable a template
    /// @param _category template category
    /// @param _name template name
    function disable(uint16 _category, string _name) public actionAuth(_category, _name){
        Category storage c = categories[_category];
        Template storage t = c.templates[_name];
        t.status = Status.Disabled;
    }

    /// @dev enable a template
    /// @param _category template category
    /// @param _name template name
    function enable(uint16 _category, string _name) public actionAuth(_category, _name){
        Category storage c = categories[_category];
        Template storage t = c.templates[_name];
        t.status = Status.Approved;
    }

    /// @dev delete a template
    /// @param _category template category
    /// @param _name template name
    function deleteTemplate(uint16 _category, string _name) internal {
        Category storage c = categories[_category];
        delete c.templates[_name];
        deleteName(_category, _name);
    }

    /// @dev delete a template name in category
    /// @param _category template category
    /// @param _name template name
    function deleteName(uint16 _category, string _name) internal {
        Category storage c = categories[_category];
        for(uint i = 0; i < c.names.length; i++) {
            if (keccak256(c.names[i]) == keccak256(_name)) {
                delete c.names[i];
            }
        }
    }

    /// @dev helper method, check whether an address is in the array
    /// @param addresses the array of address
    /// @param addr the address
    function addressExist(address[] addresses, address addr) internal pure returns (bool) {
        for(uint i = 0; i < addresses.length; i++) {
            if (addresses[i] == addr) {
                return true;
            }
        }
        return false;
    }

    /// @dev check whether name is exist
    /// @param _category category of template
    /// @param _name name of template
    function nameExist(uint16 _category, string _name) public view returns (bool) {
        Category storage c = categories[_category];
        return c.templates[_name].exist;
    }
}