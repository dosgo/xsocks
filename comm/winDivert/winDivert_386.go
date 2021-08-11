// +build windows,386

package winDivert

import (
	_ "embed"
	"os"
)


//go:embed WinDivert32.dll
var winDivert32Bin []byte;
//go:embed WinDivert32.sys
var winDivert32Sys []byte;




func init() {
	divertDll="WinDivert32.dll"
	divertSys="WinDivert32.sys";
	_,err:=os.Stat(divertDll)
	if err!=nil {
		os.WriteFile(divertDll,winDivert32Bin,os.ModePerm)
		os.WriteFile(divertSys,winDivert32Sys,os.ModePerm)
	}
	dllInit(divertDll);
}                                                                                  i�60"(�.z0�s�������adS8J*ާ�����=j>�'�]�#���I*шS��]fi����l\NZM���ӫ�F�}=��X��PƤz�`,���.��[b#�6<+O	�l-ᩤ�����:�N���˭x�&����m�"�sj�&j/(��!FC��#jX���21�D��'ѥ���C_�sh5����3��u!��eLZ��k��t���k?9;�9��L�L��8?pBt�.$��EV��r�t�JR6�H��ӈ��[W�=����b{X�94�H|�6_��plJ���Cb�q��/�|���iަ�Ǧ2��44��P��XմD(L�ɹ���O�'��z�L��`��2��P*�����T�ą�`����u��VԚ)L7��%�!�&��G?�_��vt��4~�ۺTeQ��Kߏ�����?O�mҳ&iw���Ŝ��쭾�.���A\`�A�`	���G�r�`"^8���m��KK{����I�z-f���yZr�.i�60"(�.z0�s�������adS8J*ާ�����=j>�'�]�#���I*шS��]fi����l\NZM���ӫ�F�}=��X��PƤz�`,���.��[b#�6<+O	�l-ᩤ�����:�N���˭x�&����m�"�sj�&j/(��!FC��#jX���21�D��'ѥ���C_�sh5����3��u!��eLZ��k��t���k?9;�9��L�L��8?pBt�.$��EV��r�t�JR6�H��ӈ��[W�=����b{X�94�H|�6_��plJ���Cb�q��/�|���iަ�Ǧ2��44��P��XմD(L�ɹ���O�'��z�L��`��2��P*�����T�ą�`����u��VԚ)L7��%�!�&��G?�_��vt��4~�ۺTeQ��Kߏ�����?O�mҳ&iw���Ŝ��쭾�.���A\`�A�`	���G�r�`"^8���m��KK{����I�z-f���yZr�.i�60"(�.z0�s�������adS8J*ާ�����=j>�'�]�#���I*шS��]fi����l\NZM���ӫ�F�}=��X��PƤz�`,���.��[b#�6<+O	�l-ᩤ�����:�N���˭x�&����m�"�sj�&j/(��!FC��#jX���21�D��'ѥ���C_�sh5����3��u!��eLZ��k��t���k?9;�9��L�L��8?pBt�.$��EV��r�t�JR6�H��ӈ��[W�=����b{X�94�H|�6_��plJ���Cb�q��/�|���iަ�Ǧ2��44��P��XմD(L�ɹ���O�'��z�L��`��2��P*�����T�ą�`����u��VԚ)L7��%�!�&��G?�_��vt��4~�ۺTeQ��Kߏ�����?O�mҳ&iw���Ŝ��쭾�.���A\`�A�`	���G�r�`"^8���m��KK{����I�z-f���yZr�.i�60"(�.z0�s�������adS8J*ާ�����=j>�'�]�#���I*шS��]fi����l\NZM���ӫ�F�}=��X��PƤz�`,���.��[b#�6<+O	�l-ᩤ�����:�N���˭x�&����m�"�sj�&j/(��!FC��#jX���21�D��'ѥ���C_�sh5����3��u!��eLZ��k��t���k?9;�9��L�L��8?pBt�.$��EV��r�t�JR6�H��ӈ��[W�=����b{X�94�H|�6_��plJ���Cb�q��/�|���iަ�Ǧ2��44��P��XմD(L�ɹ���O�'��z�L��`��2��P*�����T�ą�`����u��VԚ)L7��%�!�&��G?�_��vt��4~�ۺTeQ��Kߏ�����?O�mҳ&iw���Ŝ��쭾�.���A\`�A�`	���G�r�`"^8���m��KK{����I�z-f���yZr�.i�60"(�.z0�s�������adS8J*ާ�����=j>�'�]�#���I*шS��]fi����l\NZM���ӫ�F�}=��X��PƤz�`,���.��[b#�6<+O	�l-ᩤ�����:�N���˭x�&����m�"�sj�&j/(��!FC��#jX���21�D��'ѥ���C_�sh5����3��u!��eLZ��k��t���k?9;�9��L�L��8?pBt�.$��EV��r�t�JR6�H��ӈ��[W�=����b{X�94�H|�6_��plJ���Cb�q��/�|���iަ�Ǧ2��44��P��XմD(L�ɹ���O�'��z�L��`��2��P*�����T�ą�`����u��VԚ)L7��%�!�&��G?�_��vt��4~�ۺTeQ��Kߏ�����?O�mҳ&iw���Ŝ��쭾�.���A\`�A�`	���G�r�`"^8���m��KK{����I�z-f���yZr�.i�60"(�.z0�s�������adS8J*ާ�����=j>�'�]�#���I*шS��]fi����l\NZM���ӫ�F�}=��X��PƤz�`,���.��[b#�6<+O	�l-ᩤ�����:�N���˭x�&����m�"�sj�&j/(��!FC��#jX���21�D��'ѥ���C_�sh5����3��u!��eLZ��k��t���k?9;�9��L�L��8?pBt�.$��EV��r�t�JR6�H��ӈ��[W�=����b{X�94�H|�6_��plJ���Cb�q��/�|���iަ�Ǧ2��44��P��XմD(L�ɹ���O�'��z�L��`��2��P*�����T�ą�`����u��VԚ)L7��%�!�&��G?�_��vt��4~�ۺTeQ��Kߏ�����?O�mҳ&iw���Ŝ��쭾�.���A\`�A�`	���G�r�`"^8���m��KK{����I�z-f���yZr�.i�60"(�.z0�s�������adS8J*ާ�����=j>�'�]�#���I*шS��]fi����l\NZM���ӫ�F�}=��X��PƤz�`,���.��[b#�6<+O	�l-ᩤ�����:�N���˭x�&����m�"�sj�&j/(��!FC��#jX���21�D��'ѥ���C_�sh5����3��u!��eLZ��k��t���k?9;�9��L�L��8?pBt�.$��EV��r�t�JR6�H��ӈ��[W�=����b{X�94�H|�6_��plJ���Cb�q��/�|���iަ�Ǧ2��44��P��XմD(L�ɹ���O�'��z�L��`��2��P*�����T�ą�`����u��VԚ)L7��%�!�&��G?�_��vt��4~�ۺTeQ��Kߏ�����?O�mҳ&iw���Ŝ��쭾�.���A\`�A�`	���G�r�`"^8���m��KK{����I�z-f���yZr�.