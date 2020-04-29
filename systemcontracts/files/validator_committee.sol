pragma solidity 0.4.25;

import "./library/acl.sol";
import "./library/SafeMath.sol";

/**
 * censensus interface
 */
interface Consensus {
	function getValidatorBlockInfo(address validatorAddress) external view returns(uint, uint, uint);
}

/**
 * @dev This contract is designed to support Validator Committee on asimov website
 */
contract ValidatorCommittee is ACL {
	/// delegate contract address
	address constant delegateAddr = 0x63000000000000000000000000000000000000006b;
	/// initialized or not
    bool private initialized;
  	/// proposal type - for now there is one type which is to add a new asset as transaction fee
    enum ProposalType {CONFIRM_ASSETS_FOR_FEE}
    /// proposal status
    enum ProposalStatus {ONGOING, APPROVED, REJECTED}

	/// validator structure
	struct Validator {
		/// number of blocks plan to produce
        uint plannedBlocks;
        /// number of blocks actually produced
        uint actualBlocks;
        /// efficiency = actualBlocks / plannedBlocks
        uint efficiency;
		/// proposals made by the validator
		uint[] proposalIds;
		/// proposals voted
        /// proposalId => bool
        mapping(uint => bool) votedProposals;
		/// existed or not
		bool existed;
	}

	/// proposal structure
	struct Proposal {
		address proposer;
        address[] voters;
        address[] approvers;
        address[] rejecters;
        uint whichRound;
        uint percent;
        uint asset;
		/// height start to take effect
		uint effectHeight;
		uint endHeight;
        mapping(address => bool) voterRight;
		ProposalType proposalType;
        ProposalStatus status;
		bool existed;
	}

	/// asset structure
	struct AssetFee {
		uint asset;
		/// height start to take effect
		uint effectHeight;
		bool existed;
	}

	/// adress => Validator
	mapping(address => Validator) validators;
	/// proposal id => Proposal
	mapping(uint => Proposal) proposals;
	/// asset => AssetFee
	mapping(uint => AssetFee) assetFees;
	/// address => bool
	mapping(address => bool) signupValidatorsCheck;
	/// address => bool
	mapping(address => bool) chosenValidatorsCheck;
	/// asset => proposal id
	mapping(uint => uint) ongoingAssetProposalIds;

	/// signed up validators
	address[] signupValidators;
	/// validators chosen by ranking
	address[] chosenValidators;
	/// addresses allowed to start new round - for now only contains the censensus system contract
    address[] authStartNewRoundAddresses;

    /// assets accepted by validators as transaction fee
    uint[] assets;

    /// consensus system contract
    Consensus consensus;

	/// proposal index, auto incremental
    uint private proposalIndex;
	/// round in committee, note this round means the term length of validator committee instead of "round" in censensus
    uint private round;
	/// block height when last update happened
    uint private lastUpdatedHeight;

	/// round length calculated in blocks
    uint private ROUND_LENGTH;
	/// proposal length calculated in blocks
	uint private PROPOSAL_LENGTH;
    /// minimal blocks to produce if a validator wants to signup for committee
    uint private MINIMAL_SIGNUP_BLOCKS;
    /// minimal validators required in the committee
	uint private MINIMAL_VALIDATORS_COUNT;
	/// maximum validators allowed in the committee
	uint private MAXIMUM_VALIDATORS_COUNT;
    /// extra blocks needed for a new asset to take effect
    uint private MULTI_ASSET_PROPOSAL_EFFECT_HEIGHT;
    /// maximum assets allowed as transaction fee
    uint private MAXIMUM_ASSET_PROPOSAL_COUNT;

	event SignupCommitteeEvent(uint round, address validator);
	event StartCommitteeProposalEvent(uint round, uint proposalId, address proposer, ProposalType proposalType, ProposalStatus status, uint endTime);
	event ProposalVotersEvent(uint round, uint proposalId, address[] voters);
	event VoteForCommitteeEvent(uint round, uint proposalId, address voter, bool decision);
	event CommitteeProposalStatusChangeEvent(uint round, uint proposalId, ProposalStatus status, uint supportRate, uint rejectRate);
	event NewRoundEvent(uint round, uint startTime, uint endTime, address[] validators);
	event MultiAssetProposalEffectHeightEvent(uint round, uint proposalId, uint workHeight);
	event UpdateRoundBlockInfoEvent(uint round, address[] validators, uint[] plannedBlocks, uint[] actualBlocks);

	function init(address[] _validators) public {
		require(!initialized, "it is not allowed to init more than once");

        for (uint i = 0; i < _validators.length; i++) {
        	Validator storage validator = validators[_validators[i]];
        	validator.existed = true;
        	chosenValidatorsCheck[_validators[i]] = true;
        }
        chosenValidators = _validators;

        consensus = Consensus(0x63000000000000000000000000000000000000006a);
        authStartNewRoundAddresses.push(0x63000000000000000000000000000000000000006a);
        round = 1;
        lastUpdatedHeight = block.number;
		/// round length 30 days
        ROUND_LENGTH = SafeMath.mul(SafeMath.mul(30, 24), 720);
        MINIMAL_SIGNUP_BLOCKS = 10;
        MINIMAL_VALIDATORS_COUNT = 5;
        MAXIMUM_VALIDATORS_COUNT = 99;
		/// proposal length 7 days
        PROPOSAL_LENGTH = SafeMath.div(SafeMath.mul(ROUND_LENGTH, 7), 30);
        MULTI_ASSET_PROPOSAL_EFFECT_HEIGHT = 500;
        MAXIMUM_ASSET_PROPOSAL_COUNT = 99;
        initialized = true;
	}

	/**
	 * @dev sign up for validator committee
	 */
	function signup() public {
		require(!signupValidatorsCheck[msg.sender], "already signed up");

		uint plannedBlocks;
		uint actualBlocks;
		uint efficiency;
		(plannedBlocks, actualBlocks, efficiency) = consensus.getValidatorBlockInfo(msg.sender);
		require(actualBlocks >= MINIMAL_SIGNUP_BLOCKS, "not enough blocks produced");

		Validator storage validator = validators[msg.sender];
		validator.plannedBlocks = plannedBlocks;
		validator.actualBlocks = actualBlocks;
		validator.efficiency = efficiency;
		validator.existed = true;

		signupValidatorsCheck[msg.sender] = true;
		signupValidators.push(msg.sender);

		emit SignupCommitteeEvent(round, msg.sender);
	}

	/**
	 * @dev start a new round.
	 *
	 * @param validatorAddresses validator addresses
     * @param plannedBlocks blocks planned to produce by each validator
     * @param actualBlocks blocks actually produced by each validator
	 */
	function startNewRound(address[] validatorAddresses, uint[] plannedBlocks, uint[] actualBlocks)
		public
		authAddresses(authStartNewRoundAddresses)
	{
		updateValidatorsBlockInfo(validatorAddresses, plannedBlocks, actualBlocks);

		/// whether it is time to start a new round
        if (SafeMath.sub(block.number, lastUpdatedHeight) >= ROUND_LENGTH) {
            uint length = signupValidators.length;
			address[] memory tempValidators = signupValidators;

			if (length >= MINIMAL_VALIDATORS_COUNT) {
				/// empty validators of last round
				for (uint i = 0; i < chosenValidators.length; i++) {
					delete chosenValidatorsCheck[chosenValidators[i]];
				}
				delete chosenValidators;

				/// if there are more than 99 signups, we need to sort
				/// sort priority 1. blocks produced 2. efficiency if same number of blocks produced
				if (length > MAXIMUM_VALIDATORS_COUNT) {
					quickSortMemoryByActualBlocks(tempValidators, int(0), int(length-1));
				}

				uint forloopLength = length > MAXIMUM_VALIDATORS_COUNT ? MAXIMUM_VALIDATORS_COUNT : length;
	        	for (uint j = 0; j < forloopLength; j++) {
	        		chosenValidators.push(tempValidators[j]);
	        		chosenValidatorsCheck[tempValidators[j]] = true;
	        	}
			}

           	/// empty sign up information
       		for (uint k = 0; k < length; k++) {
           		delete signupValidatorsCheck[signupValidators[k]];
           	}
           	delete signupValidators;

           	round++;
            lastUpdatedHeight = block.number;
            emit NewRoundEvent(round, block.timestamp, SafeMath.add(block.timestamp, ROUND_LENGTH*5), chosenValidators);
        }
	}

	/// update block information
	function updateValidatorsBlockInfo(address[] validatorAddresses, uint[] plannedBlocks, uint[] actualBlocks) internal {
		uint validatorLength = validatorAddresses.length;
		for (uint n = 0; n < validatorLength; n++) {
			Validator storage validator = validators[validatorAddresses[n]];
			validator.plannedBlocks = SafeMath.add(validator.plannedBlocks, plannedBlocks[n]);
            validator.actualBlocks = SafeMath.add(validator.actualBlocks, actualBlocks[n]);
            validator.efficiency = SafeMath.div(SafeMath.mul(validator.actualBlocks, 100), validator.plannedBlocks);
			if (!validator.existed) {
				validator.existed = true;
			}
		}
		emit UpdateRoundBlockInfoEvent(round, validatorAddresses, plannedBlocks, actualBlocks);
	}

	/// quicksort signed up validators
	function quickSortMemoryByActualBlocks(address[] memory arr, int left, int right) private returns(address[]) {
		if (arr.length == 0) return;
		require(left >= 0 && right >= 0 && right >= left, "invalid params of left and right");
        int i = left;
        int j = right;
        if (i == j) return;
        address pivot = arr[uint(left + (right - left) / 2)];
        uint pivotValue = SafeMath.add(SafeMath.mul(validators[pivot].actualBlocks, 100), validators[pivot].efficiency);
        while (i <= j) {
            while (SafeMath.add(SafeMath.mul(validators[arr[uint(i)]].actualBlocks, 100), validators[arr[uint(i)]].efficiency) > pivotValue) i++;
            while (pivotValue > SafeMath.add(SafeMath.mul(validators[arr[uint(j)]].actualBlocks, 100), validators[arr[uint(j)]].efficiency)) j--;
            if (i <= j) {
                (arr[uint(i)], arr[uint(j)]) = (arr[uint(j)], arr[uint(i)]);
                i++;
                j--;
            }
        }
        if (left < j)
            quickSortMemoryByActualBlocks(arr, left, j);
        if (i < right)
            quickSortMemoryByActualBlocks(arr, i, right);

        return arr;
    }

	/**
	 * @dev make a proposal
	 *
	 * @param proposalType proposal type
	 * @param asset asset
	 * @return proposal id
	 */
	function startProposal(uint proposalType, uint asset) public returns(uint) {
		require(chosenValidatorsCheck[msg.sender], "not authorized");
		require(proposalType == uint(ProposalType.CONFIRM_ASSETS_FOR_FEE), "invalid proposal type");

		Validator storage validator = validators[msg.sender];
		require(validator.existed, "not authorized");

		proposalIndex = SafeMath.add(proposalIndex, 1);
		Proposal storage prop = proposals[proposalIndex];
		prop.proposer = msg.sender;
		prop.voters = chosenValidators;
		prop.whichRound = round;
		prop.effectHeight = block.number;
		prop.endHeight = SafeMath.add(prop.effectHeight, PROPOSAL_LENGTH);
		prop.status = ProposalStatus.ONGOING;
		prop.existed = true;

		if (uint(ProposalType.CONFIRM_ASSETS_FOR_FEE) == proposalType) {
			require(!assetFees[asset].existed, "asset already existed");
			require(asset & 0x10000000000000000 == 0, "only divisible asset can be proposed");
			require(assets.length < MAXIMUM_ASSET_PROPOSAL_COUNT, "not allowed anymore");
			require(!checkOngoingAssetProposal(asset), "asset is already proposed");
			prop.percent = 80;
			prop.asset = asset;
			prop.proposalType = ProposalType.CONFIRM_ASSETS_FOR_FEE;
			ongoingAssetProposalIds[asset] = proposalIndex;
		}
		for (uint i = 0; i < prop.voters.length; i++) {
            prop.voterRight[prop.voters[i]] = true;
        }
		validator.proposalIds.push(proposalIndex);

		/// proposal length, calculated in blocks
		uint proposalTimeLength = SafeMath.div(SafeMath.mul(SafeMath.mul(ROUND_LENGTH, 5), 7), 30);
		emit StartCommitteeProposalEvent(round, proposalIndex, msg.sender, prop.proposalType, prop.status, SafeMath.add(block.timestamp, proposalTimeLength));
		emit ProposalVotersEvent(round, proposalIndex, prop.voters);
		return proposalIndex;
	}

	/// check ongoing proposals to aviod proposing the same asset multiple times
	function checkOngoingAssetProposal(uint asset) internal view returns(bool) {
		uint proposalId = ongoingAssetProposalIds[asset];
		Proposal storage prop = proposals[proposalId];
		if (prop.existed && block.number < prop.endHeight && prop.status == ProposalStatus.ONGOING) {
			return true;
		}
		return false;
	}

	/**
	 * @dev vote on a proposal
	 *
	 * @param proposalId proposal id
	 * @param decision yes or no
	 */
	function vote(uint proposalId, bool decision) public {
		Proposal storage prop = proposals[proposalId];
		require(prop.existed, "the proposal does not exist");
		require(prop.whichRound == round, "invalid round value");
		require(prop.endHeight >= block.number, "invalid vote height");
		require(prop.status == ProposalStatus.ONGOING, "invalid proposal status");
		require(prop.voterRight[msg.sender], "not authorized");

		Validator storage voter = validators[msg.sender];
		require(voter.existed, "not authorized");
		require(!voter.votedProposals[proposalId], "already voted");

		emit VoteForCommitteeEvent(round, proposalId, msg.sender, decision);

		if (decision) {
			prop.approvers.push(msg.sender);
            uint supportRate = SafeMath.div(SafeMath.mul(calVotersBlocks(prop.approvers), 100), calVotersBlocks(prop.voters));
            if (supportRate >= prop.percent) {
                prop.status = ProposalStatus.APPROVED;
                if (ProposalType.CONFIRM_ASSETS_FOR_FEE == prop.proposalType) {
                	AssetFee storage newAsset = assetFees[prop.asset];
	                newAsset.asset = prop.asset;
	                newAsset.effectHeight = SafeMath.add(block.number, MULTI_ASSET_PROPOSAL_EFFECT_HEIGHT);
	                newAsset.existed = true;
	                assets.push(prop.asset);
                }
                emit CommitteeProposalStatusChangeEvent(round, proposalId, prop.status, supportRate, 0);
                emit MultiAssetProposalEffectHeightEvent(round, proposalId, newAsset.effectHeight);
            }
        } else {
        	prop.rejecters.push(msg.sender);
            uint rejectPercent = SafeMath.div(SafeMath.mul(calVotersBlocks(prop.rejecters), 100), calVotersBlocks(prop.voters));
            if (rejectPercent > SafeMath.sub(100, prop.percent)) {
                prop.status = ProposalStatus.REJECTED;
                emit CommitteeProposalStatusChangeEvent(round, proposalId, prop.status, 0, rejectPercent);
            }
        }
        voter.votedProposals[proposalId] = true;
	}

	/// calculate actual blocks produced by votors
	function calVotersBlocks(address[] voters) internal view returns(uint) {
		uint actualBlocks;
		uint length = voters.length;
		for (uint i = 0; i < length; i++) {
			actualBlocks = SafeMath.add(actualBlocks, validators[voters[i]].actualBlocks);
		}
		return actualBlocks;
	}

  	/**
  	 * @dev get the list of assets accepted as transaction fee
  	 *
  	 *	@return array of asset, array of effect height
  	 */
  	function getAssetFeeList() public view returns(uint[], uint[]) {
  		uint length = assets.length;
  		if (length == 0) {
  			return (new uint[](0), new uint[](0));
  		}

  		uint[] memory tempAssets = new uint[](length);
  		uint[] memory tempHeights = new uint[](length);
  		for (uint i = 0; i < length; i++) {
  			AssetFee storage item = assetFees[assets[i]];
  			tempAssets[i] = item.asset;
  			tempHeights[i] = item.effectHeight;
  		}
  		return (tempAssets, tempHeights);
  	}

  	/**
  	 * @dev get update block height of last round
  	 */
  	function getRoundLastUpdateHeight() public view returns(uint) {
  		return lastUpdatedHeight;
  	}

}